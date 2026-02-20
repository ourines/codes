package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"codes/internal/assistant"

	anthropic "github.com/anthropics/anthropic-sdk-go"
)

// handleAssistant handles POST /assistant
func (s *HTTPServer) handleAssistant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req AssistantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}
	if req.Text == "" {
		respondError(w, http.StatusBadRequest, "field 'text' is required")
		return
	}
	if req.SessionID == "" {
		req.SessionID = "default"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
	defer cancel()

	result, err := assistant.Run(ctx, assistant.RunOptions{
		SessionID: req.SessionID,
		Message:   req.Text,
		Model:     anthropic.Model(req.Model),
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("assistant error: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, AssistantResponse{
		Reply:     result.Reply,
		SessionID: req.SessionID,
	})
}
