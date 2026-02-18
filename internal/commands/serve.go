package commands

import (
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

	tokens := cfg.HTTPTokens
	if len(tokens) == 0 {
		ui.ShowError("HTTP server requires tokens", fmt.Errorf("no tokens configured in config.json (set 'httpTokens' field)"))
		os.Exit(1)
	}

	if cfg.HTTPBind != "" && addr == "" {
		addr = cfg.HTTPBind
	}

	if addr == "" {
		addr = ":8080"
	}

	server := httpserver.NewHTTPServer(tokens, Version)
	if err := server.ListenAndServe(addr); err != nil {
		ui.ShowError("HTTP server error", err)
		os.Exit(1)
	}
}
