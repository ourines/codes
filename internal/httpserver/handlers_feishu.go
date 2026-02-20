package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"codes/internal/assistant"
)

// feishuDedup is an in-memory event deduplication store (TTL 10 minutes).
var (
	feishuDedup   = make(map[string]time.Time)
	feishuDedupMu sync.Mutex
)

func feishuMarkSeen(eventID string) bool {
	feishuDedupMu.Lock()
	defer feishuDedupMu.Unlock()
	now := time.Now()
	for id, t := range feishuDedup {
		if now.Sub(t) > 10*time.Minute {
			delete(feishuDedup, id)
		}
	}
	if _, seen := feishuDedup[eventID]; seen {
		return false
	}
	feishuDedup[eventID] = now
	return true
}

// handleFeishuWebhook handles POST /feishu/webhook
func (s *HTTPServer) handleFeishuWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var event FeishuEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Handle URL verification challenge
	if event.Type == "url_verification" || event.Challenge != "" {
		respondJSON(w, http.StatusOK, FeishuChallengeResponse{Challenge: event.Challenge})
		return
	}

	// Only handle text messages
	if event.Header.EventType != "im.message.receive_v1" {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ignored"})
		return
	}
	if event.Event.Message.MessageType != "text" {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "non-text message"})
		return
	}

	// Deduplicate by event_id
	if event.Header.EventID != "" && !feishuMarkSeen(event.Header.EventID) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "duplicate"})
		return
	}

	// Parse text content
	var textContent FeishuTextContent
	if err := json.Unmarshal([]byte(event.Event.Message.Content), &textContent); err != nil || textContent.Text == "" {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "empty text"})
		return
	}

	// Use chat_id as session so each chat has its own conversation history.
	sessionID := event.Event.Message.ChatID
	if sessionID == "" {
		sessionID = "feishu-default"
	}
	text := strings.TrimSpace(textContent.Text)

	// Run assistant async — respond to Feishu immediately (< 3 s requirement).
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		_, _ = assistant.Run(ctx, assistant.RunOptions{
			SessionID: sessionID,
			Message:   text,
		})
		// TODO: push reply back to Feishu chat via Feishu bot API
	}()

	respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}
