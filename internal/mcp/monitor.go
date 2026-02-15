package mcpserver

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxPendingNotifications = 100

// taskNotification mirrors the notification struct from internal/agent/daemon.go.
type taskNotification struct {
	Team      string `json:"team"`
	TaskID    int    `json:"taskId"`
	Subject   string `json:"subject"`
	Status    string `json:"status"`
	Agent     string `json:"agent"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

var (
	monitorMu      sync.Mutex
	monitorStarted bool
	monitorRunning atomic.Bool

	pendingMu            sync.Mutex
	pendingNotifications []taskNotification
)

// ensureMonitorRunning starts the singleton notification monitor goroutine
// if it is not already running. The server reference is used to attempt
// best-effort MCP logging push; all notifications are also queued for
// piggyback delivery via drainPendingNotifications.
func ensureMonitorRunning(server *mcpsdk.Server) {
	monitorMu.Lock()
	defer monitorMu.Unlock()
	if monitorStarted {
		return
	}
	monitorStarted = true
	monitorRunning.Store(true)
	go runNotificationMonitor(server)
}

// drainPendingNotifications returns and clears all buffered notifications.
// Call this from any agent tool handler to piggyback unread notifications
// onto the response.
func drainPendingNotifications() []taskNotification {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	if len(pendingNotifications) == 0 {
		return nil
	}
	out := pendingNotifications
	pendingNotifications = nil
	return out
}

func runNotificationMonitor(server *mcpsdk.Server) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("monitor: cannot get home dir: %v", err)
		// Allow retry on next ensureMonitorRunning call.
		monitorMu.Lock()
		monitorStarted = false
		monitorMu.Unlock()
		monitorRunning.Store(false)
		return
	}

	dir := filepath.Join(home, ".codes", "notifications")
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // directory may not exist yet
		}

		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}

			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			var n taskNotification
			if err := json.Unmarshal(data, &n); err != nil {
				continue
			}

			// Always queue for piggyback delivery — this is the
			// reliable path. MCP logging push is best-effort only
			// because ServerSession.Log silently drops messages when
			// the client has not called SetLevel.
			pendingMu.Lock()
			if len(pendingNotifications) < maxPendingNotifications {
				pendingNotifications = append(pendingNotifications, n)
			}
			pendingMu.Unlock()

			// Best-effort: also try MCP logging push.
			tryLogToSessions(server, &n)

			// Remove the file — it is now queued (and possibly pushed).
			os.Remove(path)
		}
	}
}

// tryLogToSessions attempts to deliver a notification to all connected MCP
// sessions via ServerSession.Log as a best-effort push. Note that Log()
// silently returns nil when the client has not called SetLevel, so we cannot
// rely on the return value to determine actual delivery.
func tryLogToSessions(server *mcpsdk.Server, n *taskNotification) {
	if server == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for ss := range server.Sessions() {
		_ = ss.Log(ctx, &mcpsdk.LoggingMessageParams{
			Level:  "info",
			Logger: "agent-monitor",
			Data:   n,
		})
	}
}
