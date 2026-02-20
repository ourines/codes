package commands

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"codes/internal/config"
	"codes/internal/httpserver"
	mcpserver "codes/internal/mcp"
	"codes/internal/ui"
)

// RunServe starts the MCP server mode.
func RunServe(httpAddr string) {
	if httpAddr != "" {
		RunHTTPServer(httpAddr)
		return
	}

	if err := mcpserver.RunServer(); err != nil {
		if err.Error() != "server is closing: EOF" {
			ui.ShowError("MCP server error", err)
			os.Exit(1)
		}
	}
}

// RunHTTPServer starts the HTTP API server.
func RunHTTPServer(addr string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Failed to load config", err)
		os.Exit(1)
	}

	// Auto-generate a token if none is configured and save it for future use.
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
		fmt.Printf("Generated token: %s\n", token)
		fmt.Println("(saved to ~/.codes/config.json — use this token in the iOS app)")
	}

	if cfg.HTTPBind != "" && addr == "" {
		addr = cfg.HTTPBind
	}

	if addr == "" {
		addr = ":3456"
	}

	server := httpserver.NewHTTPServer(cfg.HTTPTokens, Version)
	if err := server.ListenAndServe(addr); err != nil {
		ui.ShowError("HTTP server error", err)
		os.Exit(1)
	}
}

// generateToken returns a random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}
