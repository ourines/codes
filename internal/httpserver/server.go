package httpserver

import (
	"log"
	"net/http"
)

// HTTPServer represents the HTTP API server
type HTTPServer struct {
	mux     *http.ServeMux
	tokens  []string
	version string
}

// NewHTTPServer creates a new HTTP server instance
func NewHTTPServer(tokens []string, version string) *HTTPServer {
	s := &HTTPServer{
		mux:     http.NewServeMux(),
		tokens:  tokens,
		version: version,
	}

	// Register routes
	s.registerRoutes()

	return s
}

// registerRoutes sets up all HTTP routes with middleware
func (s *HTTPServer) registerRoutes() {
	// Health check (no auth required)
	s.mux.HandleFunc("/health", loggingMiddleware(s.handleHealth))

	// Authenticated endpoints
	s.mux.HandleFunc("/dispatch", loggingMiddleware(s.authMiddleware(jsonContentTypeMiddleware(s.handleDispatch))))
	s.mux.HandleFunc("/tasks/", loggingMiddleware(s.authMiddleware(s.handleGetTask)))
	s.mux.HandleFunc("/teams", loggingMiddleware(s.authMiddleware(s.handleListTeams)))
	s.mux.HandleFunc("/teams/", loggingMiddleware(s.authMiddleware(s.handleGetTeam)))
}

// ListenAndServe starts the HTTP server on the given address
func (s *HTTPServer) ListenAndServe(addr string) error {
	log.Printf("[HTTP] Starting server on %s", addr)
	log.Printf("[HTTP] Registered %d valid tokens", len(s.tokens))
	return http.ListenAndServe(addr, s.mux)
}
