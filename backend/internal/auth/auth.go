// Package auth implements user authentication and token issuance.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrNotFound            = errors.New("record not found")
	ErrEmailTaken          = errors.New("email already exists")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidAccessToken  = errors.New("invalid access token")
)

// User is an authenticated application user.
type User struct {
	ID, Email, Name, AvatarURL, PasswordHash string
	CreatedAt, UpdatedAt                     time.Time
}

// RefreshSession represents a persisted refresh-token session.
type RefreshSession struct {
	ID, UserID, TokenHash string
	ExpiresAt, CreatedAt  time.Time
}

// Repository provides persistence required by the authentication service.
type Repository interface {
	CreateUserWithSession(context.Context, User, RefreshSession) error
	FindUserByEmail(context.Context, string) (User, error)
	FindUserByID(context.Context, string) (User, error)
	UpdateUserName(context.Context, string, string, time.Time) (User, error)
	UpdateUserAvatar(context.Context, string, string, time.Time) (User, error)
	CreateSession(context.Context, RefreshSession) error
	FindSessionByTokenHash(context.Context, string) (RefreshSession, error)
	RotateSession(context.Context, string, RefreshSession) error
	DeleteSessionByTokenHash(context.Context, string) error
}

// TokenPair is issued after registration, login or refresh.
type TokenPair struct {
	AccessToken, RefreshToken string
	ExpiresIn                 int64
}

// Service exposes authentication use cases.
type Service struct {
	repository                      Repository
	jwtSecret                       []byte
	accessTokenTTL, refreshTokenTTL time.Duration
	now                             func() time.Time
}

// NewService constructs an authentication service.
func NewService(repository Repository, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *Service {
	return &Service{repository: repository, jwtSecret: []byte(jwtSecret), accessTokenTTL: accessTokenTTL, refreshTokenTTL: refreshTokenTTL, now: time.Now}
}

// Register creates a user and immediately issues a token pair.
func (s *Service) Register(ctx context.Context, email, password string) (User, TokenPair, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, TokenPair{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.now().UTC()
	user := User{ID: newID(), Email: email, PasswordHash: string(hash), CreatedAt: now, UpdatedAt: now}
	pair, session, err := s.newTokenPair(user, now)
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.repository.CreateUserWithSession(ctx, user, session); err != nil {
		return User{}, TokenPair{}, err
	}
	return user, pair, nil
}

// Login verifies credentials and issues a fresh token pair.
func (s *Service) Login(ctx context.Context, email, password string) (User, TokenPair, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return User{}, TokenPair{}, ErrInvalidCredentials
	}
	user, err := s.repository.FindUserByEmail(ctx, email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return User{}, TokenPair{}, ErrInvalidCredentials
	}
	pair, session, err := s.newTokenPair(user, s.now().UTC())
	if err != nil {
		return User{}, TokenPair{}, err
	}
	if err := s.repository.CreateSession(ctx, session); err != nil {
		return User{}, TokenPair{}, err
	}
	return user, pair, nil
}

// Refresh rotates a refresh token and issues a new token pair.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	if refreshToken == "" {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	hash := tokenHash(refreshToken)
	session, err := s.repository.FindSessionByTokenHash(ctx, hash)
	if err != nil || !session.ExpiresAt.After(s.now().UTC()) {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	user, err := s.repository.FindUserByID(ctx, session.UserID)
	if err != nil {
		return TokenPair{}, ErrInvalidRefreshToken
	}
	pair, replacement, err := s.newTokenPair(user, s.now().UTC())
	if err != nil {
		return TokenPair{}, err
	}
	if err := s.repository.RotateSession(ctx, hash, replacement); err != nil {
		if errors.Is(err, ErrNotFound) {
			return TokenPair{}, ErrInvalidRefreshToken
		}
		return TokenPair{}, err
	}
	return pair, nil
}

// Logout invalidates the refresh token for the current browser session.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}

	err := s.repository.DeleteSessionByTokenHash(ctx, tokenHash(refreshToken))
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// AuthenticateAccessToken validates an access token and returns its current user.
func (s *Service) AuthenticateAccessToken(ctx context.Context, accessToken string) (User, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidAccessToken
		}
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return User{}, ErrInvalidAccessToken
	}
	userID, ok := claims["sub"].(string)
	if !ok || userID == "" {
		return User{}, ErrInvalidAccessToken
	}
	user, err := s.repository.FindUserByID(ctx, userID)
	if err != nil {
		return User{}, ErrInvalidAccessToken
	}
	return user, nil
}

// UpdateName changes the display name of an existing user.
func (s *Service) UpdateName(ctx context.Context, userID, name string) (User, error) {
	name, err := NormalizeName(name)
	if err != nil {
		return User{}, err
	}
	return s.repository.UpdateUserName(ctx, userID, name, s.now().UTC())
}

// UpdateAvatar changes the avatar URL of an existing user.
func (s *Service) UpdateAvatar(ctx context.Context, userID, avatarURL string) (User, error) {
	if avatarURL == "" {
		return User{}, errors.New("avatar URL must not be empty")
	}
	return s.repository.UpdateUserAvatar(ctx, userID, avatarURL, s.now().UTC())
}

// NormalizeEmail validates and canonicalizes an email address for lookup.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) < 3 || len(email) > 254 || !strings.Contains(email, "@") {
		return "", errors.New("email must be a valid address")
	}
	return email, nil
}

// ValidatePassword ensures the password meets the current minimum policy.
func ValidatePassword(password string) error {
	if len([]rune(password)) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	return nil
}

// NormalizeName trims and validates a user display name.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if len([]rune(name)) < 1 || len([]rune(name)) > 100 {
		return "", errors.New("name must contain from 1 to 100 characters")
	}
	return name, nil
}

func (s *Service) newTokenPair(user User, now time.Time) (TokenPair, RefreshSession, error) {
	access, err := s.signAccessToken(user, now)
	if err != nil {
		return TokenPair{}, RefreshSession{}, err
	}
	refresh, err := randomToken(32)
	if err != nil {
		return TokenPair{}, RefreshSession{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int64(s.accessTokenTTL.Seconds())}, RefreshSession{ID: newID(), UserID: user.ID, TokenHash: tokenHash(refresh), CreatedAt: now, ExpiresAt: now.Add(s.refreshTokenTTL)}, nil
}

func (s *Service) signAccessToken(user User, now time.Time) (string, error) {
	claims := jwt.MapClaims{"sub": user.ID, "email": user.Email, "name": user.Name, "avatarUrl": user.AvatarURL, "iat": now.Unix(), "exp": now.Add(s.accessTokenTTL).Unix(), "jti": newID()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}
func randomToken(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func newID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic("could not generate identifier: " + err.Error())
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", bytes[:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
}
