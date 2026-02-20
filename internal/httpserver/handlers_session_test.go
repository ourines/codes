package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codes/internal/chatsession"

	"github.com/gorilla/websocket"
)

// --- Test helpers ---

// setupSessionTest replaces the global DefaultManager with a fresh instance
// and returns a new HTTPServer. This ensures test isolation.
func setupSessionTest(t *testing.T) *HTTPServer {
	t.Helper()
	chatsession.DefaultManager = chatsession.NewSessionManager()
	return NewHTTPServer([]string{"test-token"}, "test")
}

// authedReq creates an HTTP request with a valid Bearer token and optional JSON body.
func authedReq(t *testing.T, method, path string, body interface{}) *http.Request {
	t.Helper()
	var req *http.Request
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("Failed to marshal body: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	return req
}

// doReq sends a request through the server mux and returns the recorder.
func doReq(t *testing.T, server *HTTPServer, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)
	return w
}

// decodeJSON decodes the response body into dst.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, dst interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(dst); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
}

// wsTestMsg is a local type for parsing outgoing WebSocket messages in tests.
// Mirrors chatsession.wsOutgoing but is accessible from httpserver package.
type wsTestMsg struct {
	Type    string          `json:"type"`
	Status  string          `json:"status,omitempty"`
	Event   json.RawMessage `json:"event,omitempty"`
	Message string          `json:"message,omitempty"`
}

// ============================================================
// Session CRUD Tests (no subprocess required)
// ============================================================

func TestListSessionsEmpty(t *testing.T) {
	server := setupSessionTest(t)

	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp SessionListResponse
	decodeJSON(t, w, &resp)

	if len(resp.Sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(resp.Sessions))
	}
}

func TestSessionCRUDLifecycle(t *testing.T) {
	server := setupSessionTest(t)

	// Pre-create a session via manager (stays in "creating" state, no subprocess).
	sess, err := chatsession.DefaultManager.Create("my-project", "/tmp/test-project", "sonnet")
	if err != nil {
		t.Fatalf("Manager.Create: %v", err)
	}

	// --- List: should contain the session ---
	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("List: expected 200, got %d", w.Code)
	}
	var listResp SessionListResponse
	decodeJSON(t, w, &listResp)
	if len(listResp.Sessions) != 1 {
		t.Fatalf("List: expected 1 session, got %d", len(listResp.Sessions))
	}
	if listResp.Sessions[0].ID != sess.ID {
		t.Errorf("List: ID = %q, want %q", listResp.Sessions[0].ID, sess.ID)
	}

	// --- Get by ID ---
	w = doReq(t, server, authedReq(t, http.MethodGet, "/sessions/"+sess.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Get: expected 200, got %d", w.Code)
	}
	var getResp SessionResponse
	decodeJSON(t, w, &getResp)
	if getResp.ID != sess.ID {
		t.Errorf("Get: ID mismatch")
	}
	if getResp.ProjectPath != "/tmp/test-project" {
		t.Errorf("Get: ProjectPath = %q, want /tmp/test-project", getResp.ProjectPath)
	}
	if getResp.ProjectName != "my-project" {
		t.Errorf("Get: ProjectName = %q, want my-project", getResp.ProjectName)
	}
	if getResp.Model != "sonnet" {
		t.Errorf("Get: Model = %q, want sonnet", getResp.Model)
	}
	if getResp.Status != "creating" {
		t.Errorf("Get: Status = %q, want creating", getResp.Status)
	}

	// --- Delete ---
	w = doReq(t, server, authedReq(t, http.MethodDelete, "/sessions/"+sess.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Delete: expected 200, got %d", w.Code)
	}

	// --- Get after delete: 404 ---
	w = doReq(t, server, authedReq(t, http.MethodGet, "/sessions/"+sess.ID, nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Get after delete: expected 404, got %d", w.Code)
	}

	// --- List after delete: empty ---
	w = doReq(t, server, authedReq(t, http.MethodGet, "/sessions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("List after delete: expected 200, got %d", w.Code)
	}
	decodeJSON(t, w, &listResp)
	if len(listResp.Sessions) != 0 {
		t.Errorf("List after delete: expected 0 sessions, got %d", len(listResp.Sessions))
	}
}

func TestSessionGetResponseFields(t *testing.T) {
	server := setupSessionTest(t)

	sess, _ := chatsession.DefaultManager.Create("proj", "/tmp/proj", "opus")

	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions/"+sess.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp SessionResponse
	decodeJSON(t, w, &resp)

	// Verify all expected fields are populated.
	if resp.ID == "" {
		t.Error("ID should not be empty")
	}
	if resp.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if resp.LastActiveAt.IsZero() {
		t.Error("LastActiveAt should not be zero")
	}
	if resp.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0 for new session", resp.CostUSD)
	}
	if resp.TurnCount != 0 {
		t.Errorf("TurnCount = %d, want 0 for new session", resp.TurnCount)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	server := setupSessionTest(t)

	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions/nonexistent-id", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}

	var errResp ErrorResponse
	decodeJSON(t, w, &errResp)
	if !strings.Contains(errResp.Error, "not found") {
		t.Errorf("Error should contain 'not found', got: %s", errResp.Error)
	}
}

func TestDeleteSessionNotFound(t *testing.T) {
	server := setupSessionTest(t)

	w := doReq(t, server, authedReq(t, http.MethodDelete, "/sessions/nonexistent-id", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestMultipleSessions(t *testing.T) {
	server := setupSessionTest(t)

	// Create multiple sessions.
	s1, _ := chatsession.DefaultManager.Create("p1", "/tmp/p1", "sonnet")
	s2, _ := chatsession.DefaultManager.Create("p2", "/tmp/p2", "opus")
	s3, _ := chatsession.DefaultManager.Create("p3", "/tmp/p3", "")

	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp SessionListResponse
	decodeJSON(t, w, &resp)
	if len(resp.Sessions) != 3 {
		t.Fatalf("Expected 3 sessions, got %d", len(resp.Sessions))
	}

	// Verify all IDs are present.
	ids := map[string]bool{}
	for _, s := range resp.Sessions {
		ids[s.ID] = true
	}
	for _, id := range []string{s1.ID, s2.ID, s3.ID} {
		if !ids[id] {
			t.Errorf("Session %s not found in list", id)
		}
	}

	// Delete one.
	doReq(t, server, authedReq(t, http.MethodDelete, "/sessions/"+s2.ID, nil))

	w = doReq(t, server, authedReq(t, http.MethodGet, "/sessions", nil))
	decodeJSON(t, w, &resp)
	if len(resp.Sessions) != 2 {
		t.Errorf("Expected 2 sessions after delete, got %d", len(resp.Sessions))
	}
}

// ============================================================
// Request Validation Tests
// ============================================================

func TestCreateSessionValidation(t *testing.T) {
	server := setupSessionTest(t)

	tests := []struct {
		name       string
		body       CreateSessionRequest
		wantStatus int
		wantError  string
	}{
		{
			name:       "Missing project path and name",
			body:       CreateSessionRequest{Message: "hello"},
			wantStatus: http.StatusBadRequest,
			wantError:  "either 'project_path' or 'project_name' is required",
		},
		{
			name:       "Unknown project name",
			body:       CreateSessionRequest{Message: "hello", ProjectName: "no-such-project"},
			wantStatus: http.StatusBadRequest,
			wantError:  "unknown project: no-such-project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doReq(t, server, authedReq(t, http.MethodPost, "/sessions", tt.body))
			if w.Code != tt.wantStatus {
				t.Errorf("Status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantError != "" {
				var errResp ErrorResponse
				decodeJSON(t, w, &errResp)
				if errResp.Error != tt.wantError {
					t.Errorf("Error = %q, want %q", errResp.Error, tt.wantError)
				}
			}
		})
	}
}

func TestCreateSessionInvalidJSON(t *testing.T) {
	server := setupSessionTest(t)

	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader("not-json"))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Content-Type", "application/json")

	w := doReq(t, server, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestSendMessageValidation(t *testing.T) {
	server := setupSessionTest(t)

	sess, _ := chatsession.DefaultManager.Create("", "/tmp/test", "")

	// Missing content field.
	w := doReq(t, server, authedReq(t, http.MethodPost, "/sessions/"+sess.ID+"/message",
		SessionSendMessageRequest{Content: ""}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("Empty content: expected 400, got %d", w.Code)
	}
	var errResp ErrorResponse
	decodeJSON(t, w, &errResp)
	if errResp.Error != "field 'content' is required" {
		t.Errorf("Error = %q, want 'field 'content' is required'", errResp.Error)
	}

	// Nonexistent session.
	w = doReq(t, server, authedReq(t, http.MethodPost, "/sessions/nonexistent/message",
		SessionSendMessageRequest{Content: "hello"}))
	if w.Code != http.StatusNotFound {
		t.Errorf("Nonexistent session: expected 404, got %d", w.Code)
	}
}

func TestResumeSessionValidation(t *testing.T) {
	server := setupSessionTest(t)

	sess, _ := chatsession.DefaultManager.Create("", "/tmp/test", "")

	// Missing claude_session_id.
	w := doReq(t, server, authedReq(t, http.MethodPost, "/sessions/"+sess.ID+"/resume",
		ResumeSessionRequest{}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("Empty claude_session_id: expected 400, got %d", w.Code)
	}
	var errResp ErrorResponse
	decodeJSON(t, w, &errResp)
	if errResp.Error != "field 'claude_session_id' is required" {
		t.Errorf("Error = %q, want 'field 'claude_session_id' is required'", errResp.Error)
	}

	// Nonexistent session.
	w = doReq(t, server, authedReq(t, http.MethodPost, "/sessions/nonexistent/resume",
		ResumeSessionRequest{ClaudeSessionID: "abc"}))
	if w.Code != http.StatusNotFound {
		t.Errorf("Nonexistent session: expected 404, got %d", w.Code)
	}
}

func TestInterruptSessionNotFound(t *testing.T) {
	server := setupSessionTest(t)

	w := doReq(t, server, authedReq(t, http.MethodPost, "/sessions/nonexistent/interrupt", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

func TestInterruptSessionNoProcess(t *testing.T) {
	server := setupSessionTest(t)

	// Session exists but has no subprocess (stdin is nil).
	sess, _ := chatsession.DefaultManager.Create("", "/tmp/test", "")

	w := doReq(t, server, authedReq(t, http.MethodPost, "/sessions/"+sess.ID+"/interrupt", nil))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500, got %d", w.Code)
	}

	var errResp ErrorResponse
	decodeJSON(t, w, &errResp)
	if !strings.Contains(errResp.Error, "no stdin") {
		t.Errorf("Error should mention 'no stdin', got: %s", errResp.Error)
	}
}

// ============================================================
// Method Not Allowed Tests
// ============================================================

func TestSessionMethodNotAllowed(t *testing.T) {
	server := setupSessionTest(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/sessions"},
		{http.MethodDelete, "/sessions"},
		{http.MethodPost, "/sessions/someid"},   // POST not valid for /sessions/{id}
		{http.MethodPut, "/sessions/someid"},     // PUT not valid for /sessions/{id}
		{http.MethodPatch, "/sessions/someid"},   // PATCH not valid for /sessions/{id}
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			w := doReq(t, server, authedReq(t, tt.method, tt.path, nil))
			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected 405, got %d", w.Code)
			}
		})
	}
}

func TestSessionUnknownAction(t *testing.T) {
	server := setupSessionTest(t)

	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions/someid/unknown-action", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}

	var errResp ErrorResponse
	decodeJSON(t, w, &errResp)
	if !strings.Contains(errResp.Error, "unknown session action") {
		t.Errorf("Error should mention 'unknown session action', got: %s", errResp.Error)
	}
}

// ============================================================
// WebSocket Tests (require httptest.NewServer for real TCP)
// ============================================================

// dialWS connects a WebSocket client to the given test server for session id.
func dialWS(t *testing.T, ts *httptest.Server, sessionID string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/" + sessionID + "/ws"
	header := http.Header{"Authorization": {"Bearer test-token"}}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v", err)
	}
	return conn
}

// readWSMsg reads one WebSocket message with a timeout.
func readWSMsg(t *testing.T, conn *websocket.Conn) wsTestMsg {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("WebSocket read: %v", err)
	}

	var msg wsTestMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("WebSocket unmarshal: %v", err)
	}
	return msg
}

func TestSessionWebSocketConnect(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, err := chatsession.DefaultManager.Create("test", "/tmp/test", "")
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	conn := dialWS(t, ts, sess.ID)
	defer conn.Close()

	// Should receive a session_status message immediately.
	msg := readWSMsg(t, conn)

	if msg.Type != "session_status" {
		t.Errorf("Type = %q, want session_status", msg.Type)
	}
	if msg.Status != string(chatsession.StatusCreating) {
		t.Errorf("Status = %q, want %q", msg.Status, chatsession.StatusCreating)
	}
}

func TestSessionWebSocketNotFound(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/nonexistent/ws"
	header := http.Header{"Authorization": {"Bearer test-token"}}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if conn != nil {
		conn.Close()
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	if err == nil {
		t.Fatal("Expected WebSocket dial to fail for nonexistent session")
	}

	// Server should respond with 404 before upgrading.
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("HTTP status = %d, want 404", resp.StatusCode)
	}
}

func TestSessionWebSocketMultiClient(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, err := chatsession.DefaultManager.Create("test", "/tmp/test", "")
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	// Connect client 1.
	conn1 := dialWS(t, ts, sess.ID)
	defer conn1.Close()

	// Drain initial status from client 1.
	msg1 := readWSMsg(t, conn1)
	if msg1.Type != "session_status" {
		t.Errorf("Client 1: type = %q, want session_status", msg1.Type)
	}

	// Connect client 2.
	conn2 := dialWS(t, ts, sess.ID)
	defer conn2.Close()

	// Drain initial status from client 2.
	msg2 := readWSMsg(t, conn2)
	if msg2.Type != "session_status" {
		t.Errorf("Client 2: type = %q, want session_status", msg2.Type)
	}

	// Allow goroutines to complete AddClient.
	time.Sleep(50 * time.Millisecond)

	// Verify both clients are registered.
	info := sess.Snapshot()
	if info.ClientCount != 2 {
		t.Errorf("ClientCount = %d, want 2", info.ClientCount)
	}

	// Also verify via HTTP GET endpoint.
	w := doReq(t, server, authedReq(t, http.MethodGet, "/sessions/"+sess.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET session: expected 200, got %d", w.Code)
	}
	var getResp SessionResponse
	decodeJSON(t, w, &getResp)
	if getResp.ClientCount != 2 {
		t.Errorf("HTTP ClientCount = %d, want 2", getResp.ClientCount)
	}
}

func TestSessionWebSocketUnknownMessageType(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, _ := chatsession.DefaultManager.Create("test", "/tmp/test", "")

	conn := dialWS(t, ts, sess.ID)
	defer conn.Close()

	// Drain initial status.
	readWSMsg(t, conn)

	// Send a message with an unknown type.
	unknown := map[string]string{"type": "bogus_type"}
	if err := conn.WriteJSON(unknown); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Expect an error response.
	msg := readWSMsg(t, conn)
	if msg.Type != "error" {
		t.Errorf("Type = %q, want error", msg.Type)
	}
	if !strings.Contains(msg.Message, "unknown message type") {
		t.Errorf("Message should mention 'unknown message type', got: %s", msg.Message)
	}
}

func TestSessionWebSocketInvalidJSON(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, _ := chatsession.DefaultManager.Create("test", "/tmp/test", "")

	conn := dialWS(t, ts, sess.ID)
	defer conn.Close()

	// Drain initial status.
	readWSMsg(t, conn)

	// Send malformed JSON.
	if err := conn.WriteMessage(websocket.TextMessage, []byte("{not-json}")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Expect an error response.
	msg := readWSMsg(t, conn)
	if msg.Type != "error" {
		t.Errorf("Type = %q, want error", msg.Type)
	}
	if !strings.Contains(msg.Message, "invalid JSON") {
		t.Errorf("Message should mention 'invalid JSON', got: %s", msg.Message)
	}
}

func TestSessionWebSocketEmptyUserMessage(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, _ := chatsession.DefaultManager.Create("test", "/tmp/test", "")

	conn := dialWS(t, ts, sess.ID)
	defer conn.Close()

	// Drain initial status.
	readWSMsg(t, conn)

	// Send user_message with empty content.
	emptyMsg := map[string]string{"type": "user_message", "content": ""}
	if err := conn.WriteJSON(emptyMsg); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Expect an error about missing content.
	msg := readWSMsg(t, conn)
	if msg.Type != "error" {
		t.Errorf("Type = %q, want error", msg.Type)
	}
	if !strings.Contains(msg.Message, "content is required") {
		t.Errorf("Message should mention 'content is required', got: %s", msg.Message)
	}
}

func TestSessionWebSocketClientDisconnect(t *testing.T) {
	server := setupSessionTest(t)
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	sess, _ := chatsession.DefaultManager.Create("test", "/tmp/test", "")

	conn := dialWS(t, ts, sess.ID)

	// Drain initial status.
	readWSMsg(t, conn)

	// Verify client is registered.
	info := sess.Snapshot()
	if info.ClientCount != 1 {
		t.Fatalf("Before disconnect: ClientCount = %d, want 1", info.ClientCount)
	}

	// Close the connection gracefully.
	conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	conn.Close()

	// Wait for the server-side read loop to detect disconnect.
	time.Sleep(100 * time.Millisecond)

	info = sess.Snapshot()
	if info.ClientCount != 0 {
		t.Errorf("After disconnect: ClientCount = %d, want 0", info.ClientCount)
	}
}

// ============================================================
// Path Extraction Helper Tests
// ============================================================

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/sessions/abc-123", "abc-123"},
		{"/sessions/", ""},
		{"/sessions", ""},
		{"/other/abc", ""},
		{"/sessions/abc/extra", ""},
	}

	for _, tt := range tests {
		got := extractSessionID(tt.path)
		if got != tt.want {
			t.Errorf("extractSessionID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractSessionIDFromAction(t *testing.T) {
	tests := []struct {
		path   string
		action string
		want   string
	}{
		{"/sessions/abc/ws", "ws", "abc"},
		{"/sessions/abc/interrupt", "interrupt", "abc"},
		{"/sessions/abc/resume", "resume", "abc"},
		{"/sessions/abc/message", "message", "abc"},
		{"/sessions/abc/wrong", "ws", ""},      // Action mismatch
		{"/sessions/abc", "ws", ""},             // Missing action
		{"/sessions/abc/ws/extra", "ws", ""},    // Too many parts
	}

	for _, tt := range tests {
		got := extractSessionIDFromAction(tt.path, tt.action)
		if got != tt.want {
			t.Errorf("extractSessionIDFromAction(%q, %q) = %q, want %q", tt.path, tt.action, got, tt.want)
		}
	}
}
