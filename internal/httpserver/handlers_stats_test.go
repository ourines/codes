package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStatsSummary tests GET /stats/summary.
func TestStatsSummary(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	tests := []struct {
		name   string
		period string
	}{
		{"default period", ""},
		{"today", "today"},
		{"week", "week"},
		{"month", "month"},
		{"all", "all"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := "/stats/summary"
			if tt.period != "" {
				path += "?period=" + tt.period
			}

			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("Authorization", "Bearer test-token")

			w := httptest.NewRecorder()
			server.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
			}

			var resp StatsSummaryResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			// Verify the period is set (default is "week")
			expectedPeriod := tt.period
			if expectedPeriod == "" {
				expectedPeriod = "week"
			}
			if resp.Period != expectedPeriod {
				t.Errorf("Expected period %q, got %q", expectedPeriod, resp.Period)
			}
		})
	}
}

// TestStatsSummaryMethodNotAllowed tests that POST /stats/summary returns 405.
func TestStatsSummaryMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/stats/summary", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestStatsProjects tests GET /stats/projects.
func TestStatsProjects(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/stats/projects?period=today", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp StatsProjectsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Period != "today" {
		t.Errorf("Expected period 'today', got %q", resp.Period)
	}
}

// TestStatsProjectsMethodNotAllowed tests that POST /stats/projects returns 405.
func TestStatsProjectsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/stats/projects", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestStatsModels tests GET /stats/models.
func TestStatsModels(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/stats/models", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp StatsModelsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Period defaults to "week"
	if resp.Period != "week" {
		t.Errorf("Expected period 'week', got %q", resp.Period)
	}
}

// TestStatsModelsMethodNotAllowed tests that POST /stats/models returns 405.
func TestStatsModelsMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/stats/models", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestStatsRefresh tests POST /stats/refresh.
func TestStatsRefresh(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodPost, "/stats/refresh", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var resp StatsRefreshResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Message != "stats cache refreshed" {
		t.Errorf("Expected message 'stats cache refreshed', got %q", resp.Message)
	}
}

// TestStatsRefreshMethodNotAllowed tests that GET /stats/refresh returns 405.
func TestStatsRefreshMethodNotAllowed(t *testing.T) {
	server := NewHTTPServer([]string{"test-token"}, "test")

	req := httptest.NewRequest(http.MethodGet, "/stats/refresh", nil)
	req.Header.Set("Authorization", "Bearer test-token")

	w := httptest.NewRecorder()
	server.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}
