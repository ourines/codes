package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDispatchValidation tests request validation for POST /dispatch.
func TestDispatchValidation(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	tests := []struct {
		name           string
		payload        any
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing text field",
			payload:        map[string]string{"channel": "telegram"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "field 'text' is required",
		},
		{
			name:           "Empty text field",
			payload:        map[string]string{"text": ""},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "field 'text' is required",
		},
		{
			name:           "Invalid JSON body",
			payload:        "not json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if strPayload, ok := tt.payload.(string); ok {
				body = []byte(strPayload)
			} else {
				body, err = json.Marshal(tt.payload)
				if err != nil {
					t.Fatalf("Failed to marshal payload: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/dispatch", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d (body: %s)", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedError != "" {
				var errResp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errResp.Error != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, errResp.Error)
				}
			}
		})
	}
}

// TestDispatchMethodNotAllowed tests that GET /dispatch returns 405.
func TestDispatchMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/dispatch", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestDispatchMissingContentType tests that POST /dispatch without JSON Content-Type returns 415.
func TestDispatchMissingContentType(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(map[string]string{"text": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/dispatch", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	// Deliberately omit Content-Type

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415, got %d", w.Code)
	}
}

// TestDispatchSimpleValidation tests request validation for POST /dispatch/simple.
func TestDispatchSimpleValidation(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	tests := []struct {
		name           string
		payload        any
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "Missing text field",
			payload:        map[string]string{"channel": "slack"},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "field 'text' is required",
		},
		{
			name:           "Invalid JSON body",
			payload:        "{broken",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			var err error

			if strPayload, ok := tt.payload.(string); ok {
				body = []byte(strPayload)
			} else {
				body, err = json.Marshal(tt.payload)
				if err != nil {
					t.Fatalf("Failed to marshal payload: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/dispatch/simple", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d (body: %s)", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedError != "" {
				var errResp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errResp.Error != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, errResp.Error)
				}
			}
		})
	}
}

// TestDispatchSimpleMethodNotAllowed tests that GET /dispatch/simple returns 405.
func TestDispatchSimpleMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/dispatch/simple", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestDispatchSimpleContentType tests that POST /dispatch/simple without JSON Content-Type returns 415.
func TestDispatchSimpleContentType(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(map[string]string{"text": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/dispatch/simple", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415, got %d", w.Code)
	}
}

// TestDispatchRequiresAuth tests that /dispatch endpoints require authentication.
func TestDispatchRequiresAuth(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	tests := []struct {
		name string
		path string
	}{
		{"dispatch", "/dispatch"},
		{"dispatch_simple", "/dispatch/simple"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]string{"text": "hello"})
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			// No Authorization header

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("Expected status 401, got %d", w.Code)
			}
		})
	}
}
