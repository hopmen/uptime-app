// Package monitor implements creation and retrieval of monitoring points.
package monitor

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// MaxIntervalSeconds is the largest interval supported by PostgreSQL INTEGER.
const MaxIntervalSeconds = 2147483647

var (
	ErrInvalidURL      = errors.New("укажите корректный URL с протоколом http:// или https:// без логина и пароля")
	ErrInvalidInterval = errors.New("интервал должен быть целым числом от 1 до 2147483647 секунд")
)

// Monitor is a website and its configured polling interval; no checks run yet.
type Monitor struct {
	ID              string    `json:"id"`
	UserID          string    `json:"-"`
	URL             string    `json:"url"`
	IntervalSeconds int64     `json:"intervalSeconds"`
	CreatedAt       time.Time `json:"createdAt" format:"date-time"`
}

// Repository persists monitoring points and lists them for their owner.
type Repository interface {
	CreateMonitor(context.Context, Monitor) error
	ListMonitors(context.Context, string) ([]Monitor, error)
}

// Service manages monitoring point configuration.
type Service struct{ repository Repository }

// NewService constructs a monitoring point service.
func NewService(repository Repository) *Service { return &Service{repository: repository} }

// Create validates configuration and saves a monitoring point for the user.
func (s *Service) Create(ctx context.Context, userID, rawURL string, intervalSeconds int64) (Monitor, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(u.Host, " \\") {
		return Monitor{}, ErrInvalidURL
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return Monitor{}, ErrInvalidURL
		}
	}
	if intervalSeconds < 1 || intervalSeconds > MaxIntervalSeconds {
		return Monitor{}, ErrInvalidInterval
	}
	// The UUID is generated locally; creation never makes a request to the site.
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return Monitor{}, fmt.Errorf("generate monitor ID: %w", err)
	}
	id[6] = id[6]&0x0f | 0x40
	id[8] = id[8]&0x3f | 0x80
	point := Monitor{
		ID:     fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]),
		UserID: userID, URL: rawURL, IntervalSeconds: intervalSeconds, CreatedAt: time.Now().UTC(),
	}
	if err := s.repository.CreateMonitor(ctx, point); err != nil {
		return Monitor{}, err
	}
	return point, nil
}

// List returns the user's saved monitoring points.
func (s *Service) List(ctx context.Context, userID string) ([]Monitor, error) {
	points, err := s.repository.ListMonitors(ctx, userID)
	if err != nil {
		return nil, err
	}
	if points == nil {
		points = []Monitor{}
	}
	return points, nil
}
