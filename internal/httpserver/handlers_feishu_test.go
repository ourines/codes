package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFeishuWebhookURLVerification tests the Feishu URL verification challenge.
func TestFeishuWebhookURLVerification(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	payload := FeishuEvent{
		Type:      "url_verification",
		Challenge: "test_challenge_token",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp FeishuChallengeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Challenge != "test_challenge_token" {
		t.Errorf("Expected challenge %q, got %q", "test_challenge_token", resp.Challenge)
	}
}

// TestFeishuWebhookChallengeField tests URL verification via challenge field alone.
func TestFeishuWebhookChallengeField(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	payload := FeishuEvent{
		Challenge: "another_challenge",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp FeishuChallengeResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Challenge != "another_challenge" {
		t.Errorf("Expected challenge %q, got %q", "another_challenge", resp.Challenge)
	}
}

// TestFeishuWebhookIgnoresNonMessageEvents tests that non-message event types return ignored.
func TestFeishuWebhookIgnoresNonMessageEvents(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	payload := FeishuEvent{
		Header: FeishuHeader{
			EventID:   "evt-001",
			EventType: "contact.user.updated_v3",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ignored" {
		t.Errorf("Expected status 'ignored', got %q", resp["status"])
	}
}

// TestFeishuWebhookIgnoresNonTextMessages tests that non-text message types return ignored.
func TestFeishuWebhookIgnoresNonTextMessages(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	payload := FeishuEvent{
		Header: FeishuHeader{
			EventID:   "evt-002",
			EventType: "im.message.receive_v1",
		},
		Event: FeishuEventDetail{
			Message: FeishuMessage{
				MessageType: "image",
				ChatID:      "chat-123",
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ignored" {
		t.Errorf("Expected status 'ignored', got %q", resp["status"])
	}
}

// TestFeishuWebhookDuplication tests that duplicate event_id returns duplicate status.
func TestFeishuWebhookDuplication(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	textContent, _ := json.Marshal(FeishuTextContent{Text: "hello"})
	payload := FeishuEvent{
		Header: FeishuHeader{
			EventID:   "dedup-evt-999",
			EventType: "im.message.receive_v1",
		},
		Event: FeishuEventDetail{
			Message: FeishuMessage{
				MessageType: "text",
				ChatID:      "chat-abc",
				Content:     string(textContent),
			},
		},
	}
	body, _ := json.Marshal(payload)

	// First request — accepted
	req1 := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.mux.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("First request: expected 200, got %d", w1.Code)
	}

	// Second request with same event_id — should be duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("Second request: expected 200, got %d", w2.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w2.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "duplicate" {
		t.Errorf("Expected status 'duplicate', got %q", resp["status"])
	}
}

// TestFeishuWebhookEmptyText tests that empty text content returns ignored.
func TestFeishuWebhookEmptyText(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	textContent, _ := json.Marshal(FeishuTextContent{Text: ""})
	payload := FeishuEvent{
		Header: FeishuHeader{
			EventID:   "evt-empty-text",
			EventType: "im.message.receive_v1",
		},
		Event: FeishuEventDetail{
			Message: FeishuMessage{
				MessageType: "text",
				ChatID:      "chat-xyz",
				Content:     string(textContent),
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "ignored" {
		t.Errorf("Expected status 'ignored', got %q", resp["status"])
	}
}

// TestFeishuWebhookAcceptsTextMessage tests that a valid text message returns accepted.
func TestFeishuWebhookAcceptsTextMessage(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	textContent, _ := json.Marshal(FeishuTextContent{Text: "hello world"})
	payload := FeishuEvent{
		Header: FeishuHeader{
			EventID:   "evt-accept-001",
			EventType: "im.message.receive_v1",
		},
		Event: FeishuEventDetail{
			Message: FeishuMessage{
				MessageType: "text",
				ChatID:      "chat-accept",
				Content:     string(textContent),
			},
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "accepted" {
		t.Errorf("Expected status 'accepted', got %q", resp["status"])
	}
}

// TestFeishuWebhookInvalidJSON tests that invalid JSON body returns 400.
func TestFeishuWebhookInvalidJSON(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader([]byte("{broken json")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// TestFeishuWebhookMethodNotAllowed tests that GET /feishu/webhook returns 405.
func TestFeishuWebhookMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/feishu/webhook", nil)

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestFeishuWebhookNoAuth tests that /feishu/webhook does NOT require authentication.
func TestFeishuWebhookNoAuth(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	// Challenge request — no token needed
	payload := FeishuEvent{
		Challenge: "no_auth_challenge",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/feishu/webhook", bytes.NewReader(body))
	// Deliberately no Authorization header

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (no auth required), got %d", w.Code)
	}
}
