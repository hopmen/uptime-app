package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
)

func TestRegisterThenRefresh(t *testing.T) {
	repository := newTestRepository()
	service := auth.NewService(repository, "01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	handler := New(config.Config{FrontendOrigin: "http://localhost:3000", RefreshTokenTTL: 30 * 24 * time.Hour, CookieSameSite: "lax"}, service)

	register := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.com","password":"password1"}`))
	register.Header.Set("Origin", "http://localhost:3000")
	registration := httptest.NewRecorder()
	handler.ServeHTTP(registration, register)
	if registration.Code != http.StatusCreated {
		t.Fatalf("register status = %d: %s", registration.Code, registration.Body.String())
	}
	if registration.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatal("CORS origin not set")
	}
	var response struct {
		AccessToken string `json:"accessToken"`
		ExpiresIn   int64  `json:"expiresIn"`
	}
	if err := json.NewDecoder(registration.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken == "" || response.ExpiresIn != 900 {
		t.Fatalf("invalid response: %#v", response)
	}
	cookie := registration.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.Path != "/api/v1/auth" {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}

	refresh := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refresh.AddCookie(cookie)
	refreshed := httptest.NewRecorder()
	handler.ServeHTTP(refreshed, refresh)
	if refreshed.Code != http.StatusOK {
		t.Fatalf("refresh status = %d: %s", refreshed.Code, refreshed.Body.String())
	}
	if refreshed.Result().Cookies()[0].Value == cookie.Value {
		t.Fatal("refresh token was not rotated")
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	service := auth.NewService(newTestRepository(), "01234567890123456789012345678901", 15*time.Minute, time.Hour)
	handler := New(config.Config{FrontendOrigin: "http://localhost:3000", RefreshTokenTTL: time.Hour, CookieSameSite: "lax"}, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.com","password":"short"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestLoginAndAuthenticationErrors(t *testing.T) {
	service := auth.NewService(newTestRepository(), "01234567890123456789012345678901", 15*time.Minute, time.Hour)
	handler := New(config.Config{FrontendOrigin: "http://localhost:3000", RefreshTokenTTL: time.Hour, CookieSameSite: "lax"}, service)

	firstRegistration := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.com","password":"password1"}`))
	firstResponse := httptest.NewRecorder()
	handler.ServeHTTP(firstResponse, firstRegistration)
	if firstResponse.Code != http.StatusCreated {
		t.Fatalf("first registration status = %d", firstResponse.Code)
	}

	duplicateRegistration := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.com","password":"password1"}`))
	duplicateResponse := httptest.NewRecorder()
	handler.ServeHTTP(duplicateResponse, duplicateRegistration)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", duplicateResponse.Code)
	}

	badLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"person@example.com","password":"incorrect"}`))
	badLoginResponse := httptest.NewRecorder()
	handler.ServeHTTP(badLoginResponse, badLogin)
	if badLoginResponse.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d", badLoginResponse.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"person@example.com","password":"password1"}`))
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
}

type testRepository struct {
	users    map[string]auth.User
	emails   map[string]string
	sessions map[string]auth.RefreshSession
}

func newTestRepository() *testRepository {
	return &testRepository{users: map[string]auth.User{}, emails: map[string]string{}, sessions: map[string]auth.RefreshSession{}}
}
func (r *testRepository) CreateUserWithSession(_ context.Context, user auth.User, session auth.RefreshSession) error {
	if _, ok := r.emails[user.Email]; ok {
		return auth.ErrEmailTaken
	}
	r.users[user.ID] = user
	r.emails[user.Email] = user.ID
	r.sessions[session.TokenHash] = session
	return nil
}
func (r *testRepository) FindUserByEmail(_ context.Context, email string) (auth.User, error) {
	id, ok := r.emails[email]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return r.users[id], nil
}
func (r *testRepository) FindUserByID(_ context.Context, id string) (auth.User, error) {
	user, ok := r.users[id]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return user, nil
}
func (r *testRepository) CreateSession(_ context.Context, session auth.RefreshSession) error {
	r.sessions[session.TokenHash] = session
	return nil
}
func (r *testRepository) FindSessionByTokenHash(_ context.Context, hash string) (auth.RefreshSession, error) {
	session, ok := r.sessions[hash]
	if !ok {
		return auth.RefreshSession{}, auth.ErrNotFound
	}
	return session, nil
}
func (r *testRepository) RotateSession(_ context.Context, hash string, replacement auth.RefreshSession) error {
	if _, ok := r.sessions[hash]; !ok {
		return auth.ErrNotFound
	}
	delete(r.sessions, hash)
	r.sessions[replacement.TokenHash] = replacement
	return nil
}
