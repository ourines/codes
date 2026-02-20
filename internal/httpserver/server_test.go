package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHealthEndpoint tests the /health endpoint
func TestHealthEndpoint(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", resp.Status)
	}
}

// TestAuthMiddleware tests Bearer token authentication
func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{"Valid token", "Bearer valid-token", http.StatusOK},
		{"Invalid token", "Bearer invalid-token", http.StatusUnauthorized},
		{"Missing auth header", "", http.StatusUnauthorized},
		{"Invalid format", "InvalidFormat", http.StatusUnauthorized},
	}

	server := NewHTTPServer([]string{"valid-token"}, "test")

	// Create a test handler that returns 200 if auth passes
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			server.authMiddleware(testHandler)(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestJSONContentTypeMiddleware tests JSON Content-Type validation
func TestJSONContentTypeMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		expectedStatus int
	}{
		{"POST with JSON", http.MethodPost, "application/json", http.StatusOK},
		{"POST without Content-Type", http.MethodPost, "", http.StatusUnsupportedMediaType},
		{"POST with wrong Content-Type", http.MethodPost, "text/plain", http.StatusUnsupportedMediaType},
		{"GET without Content-Type", http.MethodGet, "", http.StatusOK},
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			jsonContentTypeMiddleware(testHandler)(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

// TestMethodNotAllowed tests that endpoints reject wrong HTTP methods
func TestMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	tests := []struct {
		path   string
		method string
	}{
		{"/health", http.MethodPost},
		{"/teams", http.MethodPut},
		{"/assistant", http.MethodGet},
	}

	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Authorization", "Bearer test-token")

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405, got %d", w.Code)
			}
		})
	}
}
