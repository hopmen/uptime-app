package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"uptime-app/backend/internal/auth"
	"uptime-app/backend/internal/config"
	"uptime-app/backend/internal/monitor"
)

type testMonitorRepository struct {
	points []monitor.Monitor
	err    error
}

func (r *testMonitorRepository) CreateMonitor(_ context.Context, point monitor.Monitor) error {
	if r.err != nil {
		return r.err
	}
	r.points = append(r.points, point)
	return nil
}

func (r *testMonitorRepository) ListMonitors(_ context.Context, userID string) ([]monitor.Monitor, error) {
	if r.err != nil {
		return nil, r.err
	}
	var points []monitor.Monitor
	for i := len(r.points) - 1; i >= 0; i-- {
		if r.points[i].UserID == userID {
			points = append(points, r.points[i])
		}
	}
	return points, nil
}

func newMonitorTestHandler(t *testing.T, repository monitor.Repository) (http.Handler, string, string) {
	t.Helper()
	service := auth.NewService(newTestRepository(), "01234567890123456789012345678901", 15*time.Minute, time.Hour)
	firstUser, first, err := service.Register(context.Background(), "first@example.com", "password1")
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := service.Register(context.Background(), "second@example.com", "password1")
	if err != nil {
		t.Fatal(err)
	}
	if firstUser.ID == "" {
		t.Fatal("user ID is empty")
	}
	return New(config.Config{FrontendOrigin: "http://localhost:3000"}, service, monitor.NewService(repository)), first.AccessToken, second.AccessToken
}

func monitorRequest(handler http.Handler, method, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/v1/monitors", strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestMonitorCreateListAndOwnerIsolation(t *testing.T) {
	repository := &testMonitorRepository{}
	handler, token, otherToken := newMonitorTestHandler(t, repository)
	empty := monitorRequest(handler, http.MethodGet, token, "")
	if empty.Code != http.StatusOK || strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatalf("empty list = %d %s", empty.Code, empty.Body.String())
	}
	for _, interval := range []int64{15, 300, 7200} {
		body, err := json.Marshal(map[string]any{"url": " https://example.com/path?q=1 ", "intervalSeconds": interval})
		if err != nil {
			t.Fatal(err)
		}
		response := monitorRequest(handler, http.MethodPost, token, string(body))
		if response.Code != http.StatusCreated {
			t.Fatalf("create = %d %s", response.Code, response.Body.String())
		}
		var created monitor.Monitor
		if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
			t.Fatal(err)
		}
		if created.ID == "" || created.URL != "https://example.com/path?q=1" || created.IntervalSeconds != interval || created.CreatedAt.IsZero() {
			t.Fatalf("created = %#v", created)
		}
		if strings.Contains(response.Body.String(), "userId") {
			t.Fatal("response exposes owner ID")
		}
	}
	// A subsequent GET reads saved points, as after a page reload.
	reloaded := monitorRequest(handler, http.MethodGet, token, "")
	var points []monitor.Monitor
	if err := json.Unmarshal(reloaded.Body.Bytes(), &points); err != nil {
		t.Fatal(err)
	}
	if reloaded.Code != http.StatusOK || len(points) != 3 || points[0].IntervalSeconds != 7200 {
		t.Fatalf("list = %d %s", reloaded.Code, reloaded.Body.String())
	}
	if repository.points[0].UserID == "" || repository.points[0].ID == repository.points[1].ID {
		t.Fatal("owner or unique ID missing")
	}
	other := monitorRequest(handler, http.MethodGet, otherToken, "")
	if other.Code != http.StatusOK || strings.TrimSpace(other.Body.String()) != "[]" {
		t.Fatalf("other user's list = %d %s", other.Code, other.Body.String())
	}
}

func TestMonitorRejectsInvalidRequests(t *testing.T) {
	repository := &testMonitorRepository{}
	handler, token, _ := newMonitorTestHandler(t, repository)
	cases := map[string]string{
		"missing URL":         `{"intervalSeconds":60}`,
		"relative URL":        `{"url":"example.com","intervalSeconds":60}`,
		"FTP":                 `{"url":"ftp://example.com","intervalSeconds":60}`,
		"javascript":          `{"url":"javascript:alert(1)","intervalSeconds":60}`,
		"missing host":        `{"url":"https://","intervalSeconds":60}`,
		"credentials":         `{"url":"https://user:password@example.com","intervalSeconds":60}`,
		"invalid port":        `{"url":"https://example.com:99999","intervalSeconds":60}`,
		"missing interval":    `{"url":"https://example.com"}`,
		"zero interval":       `{"url":"https://example.com","intervalSeconds":0}`,
		"negative interval":   `{"url":"https://example.com","intervalSeconds":-1}`,
		"fractional interval": `{"url":"https://example.com","intervalSeconds":1.5}`,
		"overflow interval":   `{"url":"https://example.com","intervalSeconds":2147483648}`,
		"string interval":     `{"url":"https://example.com","intervalSeconds":"60"}`,
		"owner injection":     `{"url":"https://example.com","intervalSeconds":60,"userId":"another"}`,
		"trailing JSON":       `{"url":"https://example.com","intervalSeconds":60} {}`,
		"trailing garbage":    `{"url":"https://example.com","intervalSeconds":60} garbage`,
		"null":                `null`,
		"malformed JSON":      `{`,
		"oversized body":      `{"url":"https://example.com/` + strings.Repeat("a", 1<<20) + `","intervalSeconds":60}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			response := monitorRequest(handler, http.MethodPost, token, body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
		})
	}
	if len(repository.points) != 0 {
		t.Fatal("invalid monitor was saved")
	}
}

func TestMonitorsRequireAuthentication(t *testing.T) {
	repository := &testMonitorRepository{}
	handler, _, _ := newMonitorTestHandler(t, repository)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		for _, token := range []string{"", "invalid-token"} {
			response := monitorRequest(handler, method, token, `{"url":"https://example.com","intervalSeconds":60}`)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("%s status = %d", method, response.Code)
			}
		}
	}
	if len(repository.points) != 0 {
		t.Fatal("unauthenticated monitor was saved")
	}
}

func TestMonitorsReportStorageFailure(t *testing.T) {
	repository := &testMonitorRepository{err: errors.New("private database connection details")}
	handler, token, _ := newMonitorTestHandler(t, repository)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		response := monitorRequest(handler, method, token, `{"url":"https://example.com","intervalSeconds":60}`)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("%s status = %d", method, response.Code)
		}
		if strings.Contains(response.Body.String(), repository.err.Error()) {
			t.Fatal("private storage error exposed")
		}
	}
}
