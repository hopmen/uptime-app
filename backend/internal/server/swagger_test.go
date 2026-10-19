package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"sigs.k8s.io/yaml"

	"uptime-app/backend/internal/config"
)

func TestSwaggerContract(t *testing.T) {
	handler := New(config.Config{}, nil, nil)
	jsonResponse := httptest.NewRecorder()
	handler.ServeHTTP(jsonResponse, httptest.NewRequest(http.MethodGet, "/swagger/openapi.json", nil))
	if jsonResponse.Code != http.StatusOK || !strings.HasPrefix(jsonResponse.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("JSON contract response = %d, %s", jsonResponse.Code, jsonResponse.Header())
	}
	doc, err := openapi3.NewLoader().LoadFromData(jsonResponse.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("invalid OpenAPI contract: %v", err)
	}
	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %s", doc.OpenAPI)
	}
	for path, methods := range map[string][]string{
		"/api/v1/auth/register":       {http.MethodPost},
		"/api/v1/auth/login":          {http.MethodPost},
		"/api/v1/auth/refresh":        {http.MethodPost},
		"/api/v1/auth/logout":         {http.MethodPost},
		"/api/v1/profile":             {http.MethodGet, http.MethodPatch},
		"/api/v1/profile/avatar":      {http.MethodPost},
		"/api/v1/monitors":            {http.MethodGet, http.MethodPost},
		"/uploads/avatars/{filename}": {http.MethodGet},
	} {
		for _, method := range methods {
			if doc.Paths.Value(path) == nil || doc.Paths.Value(path).GetOperation(method) == nil {
				t.Errorf("missing operation %s %s", method, path)
			}
		}
	}
	bearer := doc.Components.SecuritySchemes["BearerAuth"].Value
	if bearer.Type != "http" || bearer.Scheme != "bearer" {
		t.Fatalf("bearer scheme = %#v", bearer)
	}
	cookie := doc.Components.SecuritySchemes["RefreshCookie"].Value
	if cookie.In != "cookie" || cookie.Name != refreshCookieName {
		t.Fatalf("refresh cookie scheme = %#v", cookie)
	}
	upload := doc.Paths.Value("/api/v1/profile/avatar").Post.RequestBody.Value
	form := upload.Content.Get("multipart/form-data").Schema.Value
	if !upload.Required || form.Properties["avatar"].Value.Format != "binary" || !reflect.DeepEqual(form.Required, []string{"avatar"}) {
		t.Fatalf("avatar multipart schema = %#v", form)
	}
	for _, path := range []string{"/swagger/openapi.yaml", "/swagger/doc.json"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, response.Code)
		}
		var actual, expected any
		if err := yaml.Unmarshal(response.Body.Bytes(), &actual); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(jsonResponse.Body.Bytes(), &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s differs from JSON contract", path)
		}
	}
}

func TestSwaggerUIServesLocalAssets(t *testing.T) {
	handler := New(config.Config{}, nil, nil)
	for _, path := range []string{"/swagger/index.html", "/swagger/swagger-ui.css", "/swagger/swagger-ui-bundle.js", "/swagger/swagger-ui-standalone-preset.js"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK || response.Body.Len() == 0 {
				t.Fatalf("status = %d, size = %d", response.Code, response.Body.Len())
			}
			if strings.HasSuffix(path, "index.html") && !strings.Contains(response.Body.String(), `url: "openapi.json"`) {
				t.Fatal("Swagger UI does not point to the embedded OpenAPI contract")
			}
		})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/", nil))
	if response.Code != http.StatusMovedPermanently || response.Header().Get("Location") != "/swagger/index.html" {
		t.Fatalf("Swagger redirect = %d, %s", response.Code, response.Header().Get("Location"))
	}
}
