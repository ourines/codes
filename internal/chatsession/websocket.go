package chatsession

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// upgrader configures the WebSocket handshake.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins; auth is handled at the HTTP layer.
	},
}

// HandleWebSocket upgrades an HTTP connection and bridges it to the ChatSession.
// It registers the client, reads incoming messages, and forwards them to Claude.
func HandleWebSocket(session *ChatSession, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[chatsession] websocket upgrade error: %v", err)
		return
	}

	session.AddClient(conn)

	// Send current status immediately.
	statusMsg := wsOutgoing{
		Type:   "session_status",
		Status: session.Snapshot().Status,
	}
	if data, err := json.Marshal(statusMsg); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}

	// Read messages from the client until disconnect.
	go func() {
		defer func() {
			session.RemoveClient(conn)
			conn.Close()
		}()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err,
					websocket.CloseGoingAway,
					websocket.CloseNormalClosure,
				) {
					log.Printf("[chatsession] ws read error for session %s: %v", session.ID, err)
				}
				return
			}

			handleClientMessage(session, conn, raw)
		}
	}()
}

// handleClientMessage processes a single incoming WebSocket message.
func handleClientMessage(session *ChatSession, conn *websocket.Conn, raw []byte) {
	var msg wsIncoming
	if err := json.Unmarshal(raw, &msg); err != nil {
		sendWSError(conn, "invalid JSON: "+err.Error())
		return
	}

	switch msg.Type {
	case "user_message":
		if msg.Content == "" {
			sendWSError(conn, "content is required for user_message")
			return
		}
		if err := session.SendMessage(msg.Content); err != nil {
			sendWSError(conn, "send message failed: "+err.Error())
		}

	case "interrupt":
		if err := session.Interrupt(); err != nil {
			sendWSError(conn, "interrupt failed: "+err.Error())
		}

	case "permission_response":
		if msg.RequestID == "" {
			sendWSError(conn, "request_id is required for permission_response")
			return
		}
		if err := session.RespondPermission(msg.RequestID, msg.Allow, msg.UpdatedInput); err != nil {
			sendWSError(conn, "permission response failed: "+err.Error())
		}

	default:
		sendWSError(conn, "unknown message type: "+msg.Type)
	}
}

// sendWSError sends an error message to a single WebSocket client.
func sendWSError(conn *websocket.Conn, message string) {
	out := wsOutgoing{
		Type:    "error",
		Message: message,
	}
	if data, err := json.Marshal(out); err == nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}
