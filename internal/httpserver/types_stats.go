package httpserver

import "codes/internal/stats"

// StatsSummaryResponse represents the stats summary API response.
type StatsSummaryResponse struct {
	Period        string             `json:"period"`
	TotalCost     float64            `json:"total_cost_usd"`
	TotalSessions int                `json:"total_sessions"`
	InputTokens   int64              `json:"input_tokens"`
	OutputTokens  int64              `json:"output_tokens"`
	CacheCreate   int64              `json:"cache_create_tokens"`
	CacheRead     int64              `json:"cache_read_tokens"`
	TopProjects   []stats.ProjectCost `json:"top_projects,omitempty"`
	TopModels     []stats.ModelCost   `json:"top_models,omitempty"`
	TopProfiles   []stats.ProfileCost `json:"top_profiles,omitempty"`
}

// StatsProjectsResponse represents the cost breakdown by project.
type StatsProjectsResponse struct {
	Period   string              `json:"period"`
	Projects []stats.ProjectCost `json:"projects"`
}

// StatsModelsResponse represents the cost breakdown by model.
type StatsModelsResponse struct {
	Period string            `json:"period"`
	Models []stats.ModelCost `json:"models"`
}

// StatsRefreshResponse represents the result of a cache refresh.
type StatsRefreshResponse struct {
	Message       string `json:"message"`
	SessionsCount int    `json:"sessions_count"`
}
