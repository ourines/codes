package httpserver

import (
	"fmt"
	"net/http"
	"time"

	"codes/internal/stats"
)

// handleStatsSummary handles GET /stats/summary?period=week
func (s *HTTPServer) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load stats cache: %v", err))
		return
	}

	cache, err = stats.RefreshIfNeeded(cache)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to refresh stats: %v", err))
		return
	}

	from, to := parsePeriod(period)
	summary := stats.GenerateSummary(cache.Sessions, from, to)

	respondJSON(w, http.StatusOK, StatsSummaryResponse{
		Period:        period,
		TotalCost:     summary.TotalCost,
		TotalSessions: summary.TotalSessions,
		InputTokens:   summary.InputTokens,
		OutputTokens:  summary.OutputTokens,
		CacheCreate:   summary.CacheCreate,
		CacheRead:     summary.CacheRead,
		TopProjects:   summary.TopProjects,
		TopModels:     summary.TopModels,
		TopProfiles:   summary.TopProfiles,
	})
}

// handleStatsProjects handles GET /stats/projects?period=week
func (s *HTTPServer) handleStatsProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load stats cache: %v", err))
		return
	}

	cache, err = stats.RefreshIfNeeded(cache)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to refresh stats: %v", err))
		return
	}

	from, to := parsePeriod(period)
	dailyStats := stats.Aggregate(cache.Sessions, from, to)
	projects := stats.ProjectBreakdown(dailyStats)

	respondJSON(w, http.StatusOK, StatsProjectsResponse{
		Period:   period,
		Projects: projects,
	})
}

// handleStatsModels handles GET /stats/models?period=week
func (s *HTTPServer) handleStatsModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load stats cache: %v", err))
		return
	}

	cache, err = stats.RefreshIfNeeded(cache)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to refresh stats: %v", err))
		return
	}

	from, to := parsePeriod(period)
	dailyStats := stats.Aggregate(cache.Sessions, from, to)
	models := stats.ModelBreakdown(dailyStats)

	respondJSON(w, http.StatusOK, StatsModelsResponse{
		Period: period,
		Models: models,
	})
}

// handleStatsRefresh handles POST /stats/refresh
func (s *HTTPServer) handleStatsRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	cache, err := stats.LoadCache()
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load stats cache: %v", err))
		return
	}

	cache, err = stats.ForceRefresh(cache)
	if err != nil {
		respondError(w, http.StatusInternalServerError, fmt.Sprintf("failed to refresh stats: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, StatsRefreshResponse{
		Message:       "stats cache refreshed",
		SessionsCount: len(cache.Sessions),
	})
}

// parsePeriod converts a period string to a time range.
func parsePeriod(period string) (from, to time.Time) {
	switch period {
	case "today":
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	case "week":
		from, to = stats.ThisWeekRange()
	case "month":
		from, to = stats.ThisMonthRange()
	case "7days":
		from, to = stats.Last7DaysRange()
	case "30days":
		from, to = stats.Last30DaysRange()
	case "all":
		// Zero values = no filter
	default:
		from, to = stats.ThisWeekRange()
	}
	return
}
