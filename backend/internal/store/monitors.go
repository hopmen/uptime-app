package store

import (
	"context"
	"time"

	"uptime-app/backend/internal/monitor"
)

type monitorModel struct {
	ID              string    `gorm:"type:uuid;primaryKey"`
	UserID          string    `gorm:"type:uuid;not null"`
	URL             string    `gorm:"not null"`
	IntervalSeconds int64     `gorm:"not null"`
	CreatedAt       time.Time `gorm:"not null"`
}

func (monitorModel) TableName() string { return "monitors" }

// CreateMonitor persists a monitoring point owned by a user.
func (r *Repository) CreateMonitor(ctx context.Context, point monitor.Monitor) error {
	return r.db.WithContext(ctx).Create(&monitorModel{
		ID: point.ID, UserID: point.UserID, URL: point.URL,
		IntervalSeconds: point.IntervalSeconds, CreatedAt: point.CreatedAt,
	}).Error
}

// ListMonitors returns only the owner's points, with newest points first.
func (r *Repository) ListMonitors(ctx context.Context, userID string) ([]monitor.Monitor, error) {
	var models []monitorModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC, id DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	points := make([]monitor.Monitor, 0, len(models))
	for _, model := range models {
		points = append(points, monitor.Monitor{
			ID: model.ID, UserID: model.UserID, URL: model.URL,
			IntervalSeconds: model.IntervalSeconds, CreatedAt: model.CreatedAt,
		})
	}
	return points, nil
}
