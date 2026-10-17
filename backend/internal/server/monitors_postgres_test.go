package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/monitor"
	"uptime-app/backend/internal/store"
)

// TestMonitorPostgres runs against an isolated schema and always rolls it back.
func TestMonitorPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	db, err := store.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(db); err != nil {
			t.Error(err)
		}
	})
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	schema := fmt.Sprintf("monitor_test_%d", time.Now().UnixNano())
	if err := tx.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec("SET LOCAL search_path TO " + schema).Error; err != nil {
		t.Fatal(err)
	}
	migrations, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(migrations) == 0 {
		t.Fatalf("read migrations: %v", err)
	}
	for _, path := range migrations {
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(migration)).Error; err != nil {
			t.Fatalf("apply %s: %v", path, err)
		}
	}
	repository := store.NewRepository(tx)
	authService := auth.NewService(repository, "01234567890123456789012345678901", time.Minute, time.Hour)
	user, pair, err := authService.Register(context.Background(), "postgres@example.com", "password1")
	if err != nil {
		t.Fatal(err)
	}
	_, otherPair, err := authService.Register(context.Background(), "other@example.com", "password1")
	if err != nil {
		t.Fatal(err)
	}
	handler := New(config.Config{}, authService, monitor.NewService(repository))
	response := monitorRequest(handler, http.MethodPost, pair.AccessToken, `{"url":"https://example.com","intervalSeconds":300}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", response.Code, response.Body.String())
	}
	var created monitor.Monitor
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// Recreate services to verify data is persisted rather than held in memory.
	repository = store.NewRepository(tx)
	handler = New(config.Config{}, authService, monitor.NewService(repository))
	listed := monitorRequest(handler, http.MethodGet, pair.AccessToken, "")
	var points []monitor.Monitor
	if err := json.Unmarshal(listed.Body.Bytes(), &points); err != nil {
		t.Fatal(err)
	}
	if listed.Code != http.StatusOK || len(points) != 1 || points[0].ID != created.ID || points[0].IntervalSeconds != 300 || points[0].URL != created.URL || points[0].CreatedAt.IsZero() {
		t.Fatalf("list persisted = %d %s", listed.Code, listed.Body.String())
	}
	other := monitorRequest(handler, http.MethodGet, otherPair.AccessToken, "")
	if other.Code != http.StatusOK || other.Body.String() != "[]\n" {
		t.Fatalf("other user = %d %s", other.Code, other.Body.String())
	}
	// Deleting the owner removes their monitoring points via the foreign key.
	if err := tx.Exec("DELETE FROM users WHERE id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := tx.Table("monitors").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("monitors left after deleting owner = %d", count)
	}
	down, err := os.ReadFile("../../migrations/000004_create_monitors.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(string(down)).Error; err != nil {
		t.Fatal(err)
	}
}
