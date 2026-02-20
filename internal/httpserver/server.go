package httpserver

import (
	"log"
	"net/http"
	"strings"
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

	// === Dispatch ===
	s.mux.HandleFunc("/dispatch", loggingMiddleware(s.authMiddleware(jsonContentTypeMiddleware(s.handleDispatch))))
	s.mux.HandleFunc("/dispatch/simple", loggingMiddleware(s.authMiddleware(jsonContentTypeMiddleware(s.handleDispatchSimple))))

	// === Projects & Profiles (Block B) ===
	s.mux.HandleFunc("/projects", loggingMiddleware(s.authMiddleware(s.handleListProjects)))
	s.mux.HandleFunc("/projects/", loggingMiddleware(s.authMiddleware(s.handleGetProject)))
	s.mux.HandleFunc("/profiles", loggingMiddleware(s.authMiddleware(s.handleListProfiles)))
	s.mux.HandleFunc("/profiles/switch", loggingMiddleware(s.authMiddleware(jsonContentTypeMiddleware(s.handleSwitchProfile))))

	// === Sessions (Block A) ===
	s.mux.HandleFunc("/sessions", loggingMiddleware(s.authMiddleware(s.routeSessions)))
	s.mux.HandleFunc("/sessions/", loggingMiddleware(s.authMiddleware(s.routeSessionByID)))

	// === Teams (Block D enhanced) ===
	s.mux.HandleFunc("/teams", loggingMiddleware(s.authMiddleware(s.routeTeams)))
	s.mux.HandleFunc("/teams/", loggingMiddleware(s.authMiddleware(s.routeTeamByName)))

	// === Tasks (direct access, existing) ===
	s.mux.HandleFunc("/tasks/", loggingMiddleware(s.authMiddleware(s.handleGetTask)))

	// === Stats (Block E) ===
	s.mux.HandleFunc("/stats/summary", loggingMiddleware(s.authMiddleware(s.handleStatsSummary)))
	s.mux.HandleFunc("/stats/projects", loggingMiddleware(s.authMiddleware(s.handleStatsProjects)))
	s.mux.HandleFunc("/stats/models", loggingMiddleware(s.authMiddleware(s.handleStatsModels)))
	s.mux.HandleFunc("/stats/refresh", loggingMiddleware(s.authMiddleware(s.handleStatsRefresh)))

	// === Workflows (Block F) ===
	s.mux.HandleFunc("/workflows", loggingMiddleware(s.authMiddleware(s.handleListWorkflows)))
	s.mux.HandleFunc("/workflows/", loggingMiddleware(s.authMiddleware(s.routeWorkflow)))
}

// --- Route dispatchers for multi-method / sub-path endpoints ---

// routeSessions dispatches GET /sessions and POST /sessions.
func (s *HTTPServer) routeSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListSessions(w, r)
	case http.MethodPost:
		jsonContentTypeMiddleware(s.handleCreateSession)(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routeSessionByID dispatches /sessions/{id}, /sessions/{id}/ws, /sessions/{id}/interrupt, etc.
func (s *HTTPServer) routeSessionByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch len(parts) {
	case 2:
		// /sessions/{id}
		switch r.Method {
		case http.MethodGet:
			s.handleGetSession(w, r)
		case http.MethodDelete:
			s.handleDeleteSession(w, r)
		default:
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case 3:
		// /sessions/{id}/{action}
		action := parts[2]
		switch action {
		case "ws":
			s.handleSessionWebSocket(w, r)
		case "interrupt":
			s.handleInterruptSession(w, r)
		case "resume":
			jsonContentTypeMiddleware(s.handleResumeSession)(w, r)
		case "message":
			jsonContentTypeMiddleware(s.handleSessionMessage)(w, r)
		default:
			respondError(w, http.StatusNotFound, "unknown session action: "+action)
		}

	default:
		respondError(w, http.StatusBadRequest, "invalid path")
	}
}

// routeTeams dispatches GET /teams and POST /teams.
func (s *HTTPServer) routeTeams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListTeams(w, r)
	case http.MethodPost:
		jsonContentTypeMiddleware(s.handleCreateTeam)(w, r)
	default:
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// routeTeamByName dispatches /teams/{name} and /teams/{name}/{sub}.
func (s *HTTPServer) routeTeamByName(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch len(parts) {
	case 2:
		// /teams/{name}
		switch r.Method {
		case http.MethodGet:
			s.handleGetTeam(w, r)
		case http.MethodDelete:
			s.handleDeleteTeam(w, r)
		default:
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		}

	case 3:
		// /teams/{name}/{sub}
		sub := parts[2]
		switch sub {
		case "tasks":
			switch r.Method {
			case http.MethodGet:
				s.handleListTeamTasks(w, r)
			case http.MethodPost:
				jsonContentTypeMiddleware(s.handleCreateTeamTask)(w, r)
			default:
				respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		case "messages":
			switch r.Method {
			case http.MethodGet:
				s.handleListTeamMessages(w, r)
			case http.MethodPost:
				jsonContentTypeMiddleware(s.handleSendTeamMessage)(w, r)
			default:
				respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		case "start":
			s.handleStartTeamAgents(w, r)
		case "stop":
			s.handleStopTeamAgents(w, r)
		case "activity":
			s.handleTeamActivity(w, r)
		default:
			respondError(w, http.StatusNotFound, "unknown team sub-resource: "+sub)
		}

	case 4:
		// /teams/{name}/tasks/{id}
		if parts[2] == "tasks" {
			s.handleUpdateTeamTask(w, r)
		} else {
			respondError(w, http.StatusNotFound, "not found")
		}

	default:
		respondError(w, http.StatusBadRequest, "invalid path")
	}
}

// routeWorkflow dispatches /workflows/{name} and /workflows/{name}/run.
func (s *HTTPServer) routeWorkflow(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	switch len(parts) {
	case 2:
		// /workflows/{name}
		s.handleGetWorkflow(w, r)
	case 3:
		// /workflows/{name}/run
		if parts[2] == "run" {
			jsonContentTypeMiddleware(s.handleRunWorkflow)(w, r)
		} else {
			respondError(w, http.StatusNotFound, "unknown workflow action: "+parts[2])
		}
	default:
		respondError(w, http.StatusBadRequest, "invalid path")
	}
}

// ListenAndServe starts the HTTP server on the given address and registers
// a Bonjour/mDNS service so iOS clients can discover it automatically.
func (s *HTTPServer) ListenAndServe(addr string) error {
	if port := parsePort(addr); port > 0 {
		stop := startMDNS(port, s.version)
		defer stop()
	}
	log.Printf("[HTTP] Starting server on %s", addr)
	log.Printf("[HTTP] Registered %d valid tokens", len(s.tokens))
	return http.ListenAndServe(addr, s.mux)
}
