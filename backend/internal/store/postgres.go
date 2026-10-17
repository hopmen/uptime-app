// Package store implements PostgreSQL persistence.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"uptime-app/backend/internal/auth"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Open establishes a PostgreSQL connection through GORM.
func Open(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{TranslateError: true})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return db, nil
}

// Close closes the GORM underlying SQL connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Repository persists users, sessions, and monitoring points in PostgreSQL.
type Repository struct{ db *gorm.DB }

// NewRepository constructs an application repository.
func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type userModel struct {
	ID           string    `gorm:"type:uuid;primaryKey"`
	Email        string    `gorm:"uniqueIndex;not null"`
	Name         string    `gorm:"not null"`
	AvatarURL    string    `gorm:"not null"`
	PasswordHash string    `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null"`
	UpdatedAt    time.Time `gorm:"not null"`
}

func (userModel) TableName() string { return "users" }

type refreshSessionModel struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	UserID    string    `gorm:"type:uuid;not null"`
	TokenHash string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (refreshSessionModel) TableName() string { return "refresh_sessions" }

// CreateUserWithSession creates a new user and its initial session atomically.
func (r *Repository) CreateUserWithSession(ctx context.Context, user auth.User, session auth.RefreshSession) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toUserModel(user)).Error; err != nil {
			if isDuplicateKey(err) {
				return auth.ErrEmailTaken
			}
			return err
		}
		return tx.Create(toSessionModel(session)).Error
	})
}

// FindUserByEmail finds a user by canonical email.
func (r *Repository) FindUserByEmail(ctx context.Context, email string) (auth.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).Where("email = ?", email).Take(&model).Error; err != nil {
		return auth.User{}, translateNotFound(err)
	}
	return fromUserModel(model), nil
}

// FindUserByID finds a user by ID.
func (r *Repository) FindUserByID(ctx context.Context, id string) (auth.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&model).Error; err != nil {
		return auth.User{}, translateNotFound(err)
	}
	return fromUserModel(model), nil
}

// UpdateUserName persists the display name and update time for a user.
func (r *Repository) UpdateUserName(ctx context.Context, id, name string, updatedAt time.Time) (auth.User, error) {
	result := r.db.WithContext(ctx).Model(&userModel{}).Where("id = ?", id).Updates(map[string]any{"name": name, "updated_at": updatedAt})
	if result.Error != nil {
		return auth.User{}, result.Error
	}
	if result.RowsAffected != 1 {
		return auth.User{}, auth.ErrNotFound
	}
	return r.FindUserByID(ctx, id)
}

// UpdateUserAvatar persists the avatar URL and update time for a user.
func (r *Repository) UpdateUserAvatar(ctx context.Context, id, avatarURL string, updatedAt time.Time) (auth.User, error) {
	result := r.db.WithContext(ctx).Model(&userModel{}).Where("id = ?", id).Updates(map[string]any{"avatar_url": avatarURL, "updated_at": updatedAt})
	if result.Error != nil {
		return auth.User{}, result.Error
	}
	if result.RowsAffected != 1 {
		return auth.User{}, auth.ErrNotFound
	}
	return r.FindUserByID(ctx, id)
}

// CreateSession persists a new refresh session.
func (r *Repository) CreateSession(ctx context.Context, session auth.RefreshSession) error {
	return r.db.WithContext(ctx).Create(toSessionModel(session)).Error
}

// FindSessionByTokenHash finds a refresh session by its irreversible token hash.
func (r *Repository) FindSessionByTokenHash(ctx context.Context, hash string) (auth.RefreshSession, error) {
	var model refreshSessionModel
	if err := r.db.WithContext(ctx).Where("token_hash = ?", hash).Take(&model).Error; err != nil {
		return auth.RefreshSession{}, translateNotFound(err)
	}
	return fromSessionModel(model), nil
}

// RotateSession replaces a session only when its old hash is still present.
func (r *Repository) RotateSession(ctx context.Context, oldHash string, replacement auth.RefreshSession) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("token_hash = ?", oldHash).Delete(&refreshSessionModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return auth.ErrNotFound
		}
		return tx.Create(toSessionModel(replacement)).Error
	})
}

// DeleteSessionByTokenHash invalidates a refresh session during logout.
func (r *Repository) DeleteSessionByTokenHash(ctx context.Context, hash string) error {
	result := r.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&refreshSessionModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func toUserModel(user auth.User) userModel {
	return userModel{ID: user.ID, Email: user.Email, Name: user.Name, AvatarURL: user.AvatarURL, PasswordHash: user.PasswordHash, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
func fromUserModel(model userModel) auth.User {
	return auth.User{ID: model.ID, Email: model.Email, Name: model.Name, AvatarURL: model.AvatarURL, PasswordHash: model.PasswordHash, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}
func toSessionModel(session auth.RefreshSession) refreshSessionModel {
	return refreshSessionModel{ID: session.ID, UserID: session.UserID, TokenHash: session.TokenHash, ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt}
}
func fromSessionModel(model refreshSessionModel) auth.RefreshSession {
	return auth.RefreshSession{ID: model.ID, UserID: model.UserID, TokenHash: model.TokenHash, ExpiresAt: model.ExpiresAt, CreatedAt: model.CreatedAt}
}
func translateNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return auth.ErrNotFound
	}
	return err
}
func isDuplicateKey(err error) bool { return errors.Is(err, gorm.ErrDuplicatedKey) }
