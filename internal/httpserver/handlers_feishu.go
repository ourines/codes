package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"codes/internal/dispatch"
)

// feishuDedup is an in-memory event deduplication store (TTL 10 minutes).
var (
	feishuDedup   = make(map[string]time.Time)
	feishuDedupMu sync.Mutex
)

func feishuMarkSeen(eventID string) bool {
	feishuDedupMu.Lock()
	defer feishuDedupMu.Unlock()
	// Clean expired entries
	now := time.Now()
	for id, t := range feishuDedup {
		if now.Sub(t) > 10*time.Minute {
			delete(feishuDedup, id)
		}
	}
	if _, seen := feishuDedup[eventID]; seen {
		return false // already processed
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

	// Dispatch async — respond to Feishu immediately (< 3s timeout requirement)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, _ = dispatch.Dispatch(ctx, dispatch.DispatchOptions{
			UserInput: textContent.Text,
			Channel:   "feishu",
			ChatID:    event.Event.Message.ChatID,
		})
	}()

	respondJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}
