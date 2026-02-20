package chatsession

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Start spawns a Claude subprocess. If firstMessage is non-empty it is sent
// immediately and the session becomes busy; otherwise the session waits in
// ready state for the user to send the first message via SendMessage/WebSocket.
func (s *ChatSession) Start(firstMessage string) error {
	s.mu.Lock()
	if s.Status != StatusCreating {
		s.mu.Unlock()
		return fmt.Errorf("session %s is in state %s, expected creating", s.ID, s.Status)
	}
	s.mu.Unlock()

	stdin, stdout, cmd, err := spawnClaude(s.ProjectPath, s.Model, "")
	if err != nil {
		s.mu.Lock()
		s.Status = StatusClosed
		s.mu.Unlock()
		return fmt.Errorf("spawn claude: %w", err)
	}

	s.mu.Lock()
	s.process = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.done = make(chan struct{})
	s.Status = StatusReady
	s.LastActiveAt = time.Now()
	s.mu.Unlock()

	// Start the read pump in background (reads Claude stdout, broadcasts to clients).
	go s.readPump()

	// Send the first user message only if provided.
	if firstMessage != "" {
		// Store for replay so reconnecting clients see the initial question.
		if evt, err := json.Marshal(map[string]string{"type": "user", "content": firstMessage}); err == nil {
			s.mu.Lock()
			s.messages = append(s.messages, evt)
			s.mu.Unlock()
		}
		if err := s.writeUserMessage(firstMessage); err != nil {
			s.Close()
			return fmt.Errorf("send first message: %w", err)
		}
		s.mu.Lock()
		s.Status = StatusBusy
		s.mu.Unlock()
	}

	return nil
}

// Resume restarts a session from a previous Claude session ID.
func (s *ChatSession) Resume(claudeSessionID string) error {
	s.mu.Lock()
	if s.Status != StatusCreating {
		s.mu.Unlock()
		return fmt.Errorf("session %s is in state %s, expected creating", s.ID, s.Status)
	}
	s.mu.Unlock()

	stdin, stdout, cmd, err := spawnClaude(s.ProjectPath, s.Model, claudeSessionID)
	if err != nil {
		s.mu.Lock()
		s.Status = StatusClosed
		s.mu.Unlock()
		return fmt.Errorf("spawn claude for resume: %w", err)
	}

	s.mu.Lock()
	s.process = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.done = make(chan struct{})
	s.ClaudeSessionID = claudeSessionID
	s.Status = StatusReady
	s.LastActiveAt = time.Now()
	s.mu.Unlock()

	go s.readPump()
	return nil
}

// SendMessage writes a user message to the Claude stdin for multi-turn conversation.
func (s *ChatSession) SendMessage(content string) error {
	s.mu.Lock()
	if s.Status == StatusClosed {
		s.mu.Unlock()
		return fmt.Errorf("session %s is closed", s.ID)
	}
	needsRespawn := s.stdin == nil
	claudeSessionID := s.ClaudeSessionID
	s.mu.Unlock()

	// Claude process exits after each turn; transparently restart it with --resume.
	if needsRespawn {
		if err := s.respawn(claudeSessionID); err != nil {
			return fmt.Errorf("respawn for new turn: %w", err)
		}
	}

	s.mu.Lock()
	// Store user message for replay so reconnecting clients see the full conversation.
	if evt, err := json.Marshal(map[string]string{"type": "user", "content": content}); err == nil {
		s.messages = append(s.messages, evt)
	}
	s.Status = StatusBusy
	s.TurnCount++
	s.LastActiveAt = time.Now()
	s.mu.Unlock()

	s.broadcastStatus(StatusBusy)

	if err := s.writeUserMessage(content); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	return nil
}

// respawn starts a new Claude subprocess resuming the given session ID.
// Called when the previous process has exited after completing a turn.
func (s *ChatSession) respawn(claudeSessionID string) error {
	stdin, stdout, cmd, err := spawnClaude(s.ProjectPath, s.Model, claudeSessionID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.process = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.done = make(chan struct{})
	s.mu.Unlock()

	go s.readPump()
	return nil
}

// Interrupt sends an interrupt control request to Claude.
func (s *ChatSession) Interrupt() error {
	s.mu.Lock()
	if s.stdin == nil {
		s.mu.Unlock()
		return fmt.Errorf("session %s has no stdin", s.ID)
	}
	s.mu.Unlock()

	req := stdinControlRequest{
		Type:      "control_request",
		RequestID: fmt.Sprintf("interrupt-%d", time.Now().UnixNano()),
		Request:   stdinControlPayload{Subtype: "interrupt"},
	}
	return s.writeJSON(req)
}

// RespondPermission sends a permission response to Claude.
func (s *ChatSession) RespondPermission(requestID string, allow bool, updatedInput json.RawMessage) error {
	s.mu.Lock()
	if s.stdin == nil {
		s.mu.Unlock()
		return fmt.Errorf("session %s has no stdin", s.ID)
	}
	s.mu.Unlock()

	behavior := "deny"
	if allow {
		behavior = "allow"
	}

	resp := stdinControlResponse{
		Type: "control_response",
		Response: stdinControlResponseInner{
			Subtype:   "success",
			RequestID: requestID,
			Response: stdinControlResponseBehavior{
				Behavior: behavior,
			},
		},
	}

	if allow && updatedInput != nil {
		resp.Response.Response.UpdatedInput = updatedInput
	}
	if !allow {
		resp.Response.Response.Message = "User denied permission"
	}

	return s.writeJSON(resp)
}

// Close terminates the Claude subprocess and cleans up.
func (s *ChatSession) Close() error {
	s.mu.Lock()
	if s.Status == StatusClosed {
		s.mu.Unlock()
		return nil
	}
	s.Status = StatusClosed
	process := s.process
	stdin := s.stdin
	s.mu.Unlock()

	// Close stdin first to signal EOF.
	if stdin != nil {
		stdin.Close()
	}

	// Kill process if still running.
	if process != nil && process.Process != nil {
		process.Process.Kill()
		process.Wait()
	}

	// Notify all connected clients.
	s.broadcastStatus(StatusClosed)

	// Close all client connections.
	s.mu.Lock()
	for conn := range s.clients {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"))
		conn.Close()
	}
	s.clients = nil
	s.mu.Unlock()

	return nil
}

// AddClient registers a WebSocket connection to receive events.
func (s *ChatSession) AddClient(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.clients == nil {
		s.clients = make(map[*websocket.Conn]bool)
	}
	s.clients[conn] = true

	// Replay cached messages so the client can catch up.
	for _, msg := range s.messages {
		out := wsOutgoing{Type: "claude_event", Event: msg}
		if data, err := json.Marshal(out); err == nil {
			conn.WriteMessage(websocket.TextMessage, data)
		}
	}
}

// RemoveClient unregisters a WebSocket connection.
func (s *ChatSession) RemoveClient(conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, conn)
}

// Done returns a channel that is closed when the readPump exits (subprocess ended).
func (s *ChatSession) Done() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return s.done
}

// --- internal helpers ---

// writeUserMessage encodes a user message and writes it to Claude's stdin.
func (s *ChatSession) writeUserMessage(content string) error {
	msg := stdinUserMessage{
		Type: "user",
		Message: stdinMsgContent{
			Role:    "user",
			Content: content,
		},
	}
	return s.writeJSON(msg)
}

// writeJSON marshals v to JSON and writes it as a line to Claude stdin.
func (s *ChatSession) writeJSON(v any) error {
	s.mu.Lock()
	w := s.stdin
	s.mu.Unlock()

	if w == nil {
		return fmt.Errorf("stdin closed")
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	_, err = w.Write(data)
	return err
}

// readPump reads lines from Claude's stdout and broadcasts them to all clients.
// When stdout closes (subprocess exits), it sets status to ready or closed.
func (s *ChatSession) readPump() {
	defer func() {
		s.mu.Lock()
		if s.done != nil {
			close(s.done)
		}
		s.mu.Unlock()
	}()

	scanner := bufio.NewScanner(s.stdout)
	// Allow up to 1MB per line for large Claude events.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Cache the raw event.
		raw := make(json.RawMessage, len(line))
		copy(raw, line)

		s.mu.Lock()
		s.messages = append(s.messages, raw)
		s.LastActiveAt = time.Now()
		s.mu.Unlock()

		// Extract session_id and detect "result" type for status tracking.
		s.processEvent(raw)

		// Broadcast to all WebSocket clients.
		s.broadcast(wsOutgoing{Type: "claude_event", Event: raw})
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[chatsession] readPump error for session %s: %v", s.ID, err)
	}

	// Subprocess stdout closed — clear IO handles so SendMessage knows to respawn.
	s.mu.Lock()
	wasStatus := s.Status
	if wasStatus != StatusClosed {
		s.Status = StatusReady
		// Wait for the process to exit so the next respawn doesn't race.
		if s.process != nil {
			proc := s.process
			s.mu.Unlock()
			proc.Wait()
			s.mu.Lock()
		}
		s.stdin = nil
		s.stdout = nil
		s.process = nil
	}
	s.mu.Unlock()

	if wasStatus != StatusClosed {
		s.broadcastStatus(StatusReady)
	}
}

// processEvent inspects a raw Claude event for metadata (session_id, result type, cost).
func (s *ChatSession) processEvent(raw json.RawMessage) {
	var event struct {
		Type      string  `json:"type"`
		SessionID string  `json:"session_id"`
		CostUSD   float64 `json:"cost_usd"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if event.SessionID != "" {
		s.ClaudeSessionID = event.SessionID
	}
	if event.CostUSD > 0 {
		s.CostUSD = event.CostUSD
	}
	if event.Type == "result" {
		s.Status = StatusReady
	}
}

// broadcast sends a message to all connected WebSocket clients.
func (s *ChatSession) broadcast(msg wsOutgoing) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[chatsession] broadcast marshal error: %v", err)
		return
	}

	s.mu.Lock()
	clients := make([]*websocket.Conn, 0, len(s.clients))
	for c := range s.clients {
		clients = append(clients, c)
	}
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, conn := range clients {
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("[chatsession] write to client error: %v", err)
				s.RemoveClient(c)
			}
		}(conn)
	}
	wg.Wait()
}

// broadcastStatus sends a session_status message to all clients.
func (s *ChatSession) broadcastStatus(status SessionStatus) {
	s.broadcast(wsOutgoing{
		Type:   "session_status",
		Status: status,
	})
}
