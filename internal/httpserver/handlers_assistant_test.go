package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAssistantMethodNotAllowed tests that GET /assistant returns 405.
func TestAssistantMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/assistant", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestAssistantRequiresAuth tests that POST /assistant without token returns 401.
func TestAssistantRequiresAuth(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(AssistantRequest{Text: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/assistant", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

// TestAssistantRequiresJSONContentType tests that POST /assistant without JSON content type returns 415.
func TestAssistantRequiresJSONContentType(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(AssistantRequest{Text: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/assistant", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415, got %d", w.Code)
	}
}

// TestAssistantMissingText tests that POST /assistant without text field returns 400.
func TestAssistantMissingText(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(AssistantRequest{SessionID: "sess-1"})
	req := httptest.NewRequest(http.MethodPost, "/assistant", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d (body: %s)", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errResp.Error != "field 'text' is required" {
		t.Errorf("Expected error 'field 'text' is required', got %q", errResp.Error)
	}
}

// TestAssistantEmptyText tests that POST /assistant with empty text returns 400.
func TestAssistantEmptyText(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	body, _ := json.Marshal(AssistantRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/assistant", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d (body: %s)", w.Code, w.Body.String())
	}
}

// TestAssistantInvalidJSON tests that POST /assistant with invalid JSON returns 400.
func TestAssistantInvalidJSON(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/assistant", bytes.NewReader([]byte("{broken")))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
