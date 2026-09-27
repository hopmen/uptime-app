package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestRegisterNormalizesEmailAndIssuesJWT(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository, "01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	user, pair, err := service.Register(context.Background(), "  PERSON@Example.COM ", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if user.Email != "person@example.com" {
		t.Fatalf("email = %q", user.Email)
	}
	if pair.RefreshToken == "" || pair.AccessToken == "" || pair.ExpiresIn != 900 {
		t.Fatalf("invalid token pair: %#v", pair)
	}
	if len(repository.sessions) != 1 {
		t.Fatalf("sessions = %d, want 1", len(repository.sessions))
	}
	parsed, err := jwt.Parse(pair.AccessToken, func(token *jwt.Token) (any, error) { return []byte("01234567890123456789012345678901"), nil })
	if err != nil || !parsed.Valid {
		t.Fatalf("parse JWT: %v", err)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if claims["sub"] != user.ID || claims["email"] != user.Email {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestRefreshRotatesTokenAndRejectsReuse(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository, "01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	_, initial, err := service.Register(context.Background(), "person@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	rotated, err := service.Refresh(context.Background(), initial.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshToken == initial.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	if _, err := service.Refresh(context.Background(), initial.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("reused token error = %v", err)
	}
}

func TestLogoutInvalidatesRefreshToken(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository, "01234567890123456789012345678901", 15*time.Minute, 30*24*time.Hour)
	_, pair, err := service.Register(context.Background(), "person@example.com", "password1")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := service.Logout(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("refresh after logout error = %v", err)
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Fatal("expected short password error")
	}
	if err := ValidatePassword("достаточно"); err != nil {
		t.Fatalf("unicode password: %v", err)
	}
}

type memoryRepository struct {
	users        map[string]User
	usersByEmail map[string]string
	sessions     map[string]RefreshSession
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{users: map[string]User{}, usersByEmail: map[string]string{}, sessions: map[string]RefreshSession{}}
}
func (r *memoryRepository) CreateUserWithSession(_ context.Context, user User, session RefreshSession) error {
	if _, exists := r.usersByEmail[user.Email]; exists {
		return ErrEmailTaken
	}
	r.users[user.ID] = user
	r.usersByEmail[user.Email] = user.ID
	r.sessions[session.TokenHash] = session
	return nil
}
func (r *memoryRepository) FindUserByEmail(_ context.Context, email string) (User, error) {
	id, ok := r.usersByEmail[email]
	if !ok {
		return User{}, ErrNotFound
	}
	return r.users[id], nil
}
func (r *memoryRepository) FindUserByID(_ context.Context, id string) (User, error) {
	user, ok := r.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}
func (r *memoryRepository) UpdateUserName(_ context.Context, id, name string, updatedAt time.Time) (User, error) {
	user, ok := r.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	user.Name = name
	user.UpdatedAt = updatedAt
	r.users[id] = user
	return user, nil
}
func (r *memoryRepository) CreateSession(_ context.Context, session RefreshSession) error {
	r.sessions[session.TokenHash] = session
	return nil
}
func (r *memoryRepository) FindSessionByTokenHash(_ context.Context, hash string) (RefreshSession, error) {
	session, ok := r.sessions[hash]
	if !ok {
		return RefreshSession{}, ErrNotFound
	}
	return session, nil
}
func (r *memoryRepository) RotateSession(_ context.Context, oldHash string, replacement RefreshSession) error {
	if _, ok := r.sessions[oldHash]; !ok {
		return ErrNotFound
	}
	delete(r.sessions, oldHash)
	r.sessions[replacement.TokenHash] = replacement
	return nil
}
func (r *memoryRepository) DeleteSessionByTokenHash(_ context.Context, hash string) error {
	if _, ok := r.sessions[hash]; !ok {
		return ErrNotFound
	}
	delete(r.sessions, hash)
	return nil
}
