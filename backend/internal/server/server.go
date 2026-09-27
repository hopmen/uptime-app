// Package server exposes the HTTP API.
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
)

const refreshCookieName = "refresh_token"

// New builds the application HTTP handler.
func New(cfg config.Config, authService *auth.Service) http.Handler {
	handler := &authHandler{service: authService, config: cfg}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/register", handler.register)
	mux.HandleFunc("POST /api/v1/auth/login", handler.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", handler.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", handler.logout)
	mux.HandleFunc("GET /api/v1/profile", handler.profile)
	mux.HandleFunc("PATCH /api/v1/profile", handler.updateProfile)
	return cors(cfg.FrontendOrigin, mux)
}

type authHandler struct {
	service *auth.Service
	config  config.Config
}
type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}
type updateProfileRequest struct {
	Name string `json:"name"`
}
type tokenResponse struct {
	User        userResponse `json:"user"`
	AccessToken string       `json:"accessToken"`
	ExpiresIn   int64        `json:"expiresIn"`
}
type refreshResponse struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"`
}
type errorResponse struct {
	Error string `json:"error"`
}

func (h *authHandler) register(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	user, pair, err := h.service.Register(r.Context(), request.Email, request.Password)
	if err != nil {
		if errors.Is(err, auth.ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email is already registered")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid registration data")
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusCreated, tokenResponse{User: userResponseFromUser(user), AccessToken: pair.AccessToken, ExpiresIn: pair.ExpiresIn})
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	user, pair, err := h.service.Login(r.Context(), request.Email, request.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusOK, tokenResponse{User: userResponseFromUser(user), AccessToken: pair.AccessToken, ExpiresIn: pair.ExpiresIn})
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	pair, err := h.service.Refresh(r.Context(), cookie.Value)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	h.setRefreshCookie(w, pair.RefreshToken)
	writeJSON(w, http.StatusOK, refreshResponse{AccessToken: pair.AccessToken, ExpiresIn: pair.ExpiresIn})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			writeError(w, http.StatusInternalServerError, "could not log out")
			return
		}
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandler) profile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, userResponseFromUser(user))
}

func (h *authHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authenticatedUser(w, r)
	if !ok {
		return
	}
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request updateProfileRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	updatedUser, err := h.service.UpdateName(r.Context(), user.ID, request.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, "name must contain from 1 to 100 characters")
		return
	}
	writeJSON(w, http.StatusOK, userResponseFromUser(updatedUser))
}

func (h *authHandler) authenticatedUser(w http.ResponseWriter, r *http.Request) (auth.User, bool) {
	accessToken, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || accessToken == "" {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return auth.User{}, false
	}
	user, err := h.service.AuthenticateAccessToken(r.Context(), accessToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid access token")
		return auth.User{}, false
	}
	return user, true
}

func userResponseFromUser(user auth.User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email, Name: user.Name, CreatedAt: user.CreatedAt}
}

func (h *authHandler) setRefreshCookie(w http.ResponseWriter, refreshToken string) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: refreshToken, Path: "/api/v1/auth", HttpOnly: true, Secure: h.config.CookieSecure, SameSite: sameSite(h.config.CookieSameSite), MaxAge: int(h.config.RefreshTokenTTL.Seconds()), Expires: time.Now().Add(h.config.RefreshTokenTTL)})
}

func (h *authHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/api/v1/auth", HttpOnly: true, Secure: h.config.CookieSecure, SameSite: sameSite(h.config.CookieSameSite), MaxAge: -1, Expires: time.Unix(0, 0)})
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentialsRequest, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request credentialsRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return credentialsRequest{}, false
	}
	return request, true
}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}
func sameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") == origin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			if r.Header.Get("Origin") != origin {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, PATCH, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
