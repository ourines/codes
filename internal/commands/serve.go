package commands

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"codes/internal/assistant"
	"codes/internal/assistant/scheduler"
	"codes/internal/config"
	"codes/internal/httpserver"
	mcpserver "codes/internal/mcp"
	"codes/internal/ui"
)

// RunServe is the single entry point for `codes serve`.
//
// Always starts (single port :3456):
//   - HTTP REST server + assistant scheduler
//   - SSE MCP handler mounted at /mcp/
//   - stdio MCP when stdin is a pipe (e.g. spawned by Claude Code)
func RunServe() {
	// Detect whether we were spawned with a pipe on stdin (Claude Code MCP mode).
	stdioMCP := isStdinPipe()

	// When stdio MCP is active, redirect all log/print output to stderr so we
	// don't corrupt the JSON-RPC stream on stdout.
	var out io.Writer = os.Stdout
	if stdioMCP {
		out = os.Stderr
		log.SetOutput(os.Stderr)
	}

	// ── Config & auth token ───────────────────────────────────────────────────
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Failed to load config", err)
		os.Exit(1)
	}
	if len(cfg.HTTPTokens) == 0 {
		token, err := generateToken()
		if err != nil {
			ui.ShowError("Failed to generate token", err)
			os.Exit(1)
		}
		cfg.HTTPTokens = []string{token}
		if saveErr := config.SaveConfig(cfg); saveErr != nil {
			fmt.Fprintf(os.Stderr, "[warn] could not save generated token: %v\n", saveErr)
		}
		fmt.Fprintf(out, "Generated token: %s\n", token)
		fmt.Fprintf(out, "(saved to ~/.codes/config.json — use this token in iOS / API clients)\n")
	}

	httpAddr := cfg.HTTPBind
	if httpAddr == "" {
		httpAddr = ":3456"
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Scheduler (goroutine) ─────────────────────────────────────────────────
	sched := startScheduler(out)
	if sched != nil {
		defer sched.Stop()
		assistant.SetScheduler(sched)
	}

	// ── HTTP REST server + SSE MCP (goroutine) ───────────────────────────────
	fmt.Fprintf(out, "HTTP + MCP SSE server listening on %s\n", httpAddr)
	httpServer := httpserver.NewHTTPServer(cfg.HTTPTokens, Version)
	httpServer.Handle("/mcp/", mcpserver.NewSSEHandler())
	go func() {
		if err := httpServer.ListenAndServe(httpAddr); err != nil && err.Error() != "http: Server closed" {
			fmt.Fprintf(os.Stderr, "[http] error: %v\n", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_ = httpServer.Shutdown(shutCtx)
	}()

	// ── stdio MCP (blocking) or wait for signal ───────────────────────────────
	if stdioMCP {
		// Stdout is now exclusively for the MCP JSON-RPC protocol.
		if err := mcpserver.RunServer(); err != nil && err.Error() != "server is closing: EOF" {
			fmt.Fprintf(os.Stderr, "[mcp-stdio] error: %v\n", err)
			os.Exit(1)
		}
	} else {
		<-ctx.Done()
		fmt.Fprintf(out, "\nShutting down...\n")
	}
}

// isStdinPipe returns true when stdin is a pipe or file (not a terminal),
// i.e. codes was spawned by another process feeding it data.
func isStdinPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// startScheduler initialises and starts the assistant scheduler.
func startScheduler(out io.Writer) *scheduler.Scheduler {
	sched := scheduler.New(func(sessionID, message string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := assistant.Run(ctx, assistant.RunOptions{
			SessionID: sessionID,
			Message:   message,
		})
		if err != nil {
			log.Printf("[scheduler] trigger error (session=%s): %v", sessionID, err)
			return
		}
		log.Printf("[scheduler] reply [%s]: %s", sessionID, result.Reply)
	})
	if err := sched.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "[scheduler] start error: %v\n", err)
		return nil
	}
	fmt.Fprintf(out, "Assistant scheduler started\n")
	return sched
}

// generateToken returns a random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
