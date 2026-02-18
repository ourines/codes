package stats

import "time"

// SessionRecord represents the parsed stats of a single Claude session.
type SessionRecord struct {
	SessionID         string        `json:"sessionId"`
	Project           string        `json:"project"`     // codes project alias (e.g. "codes"), falls back to path
	ProjectPath       string        `json:"projectPath"` // full filesystem path
	Profile           string        `json:"profile"`     // API profile name (config.Default at scan time), falls back to "unknown"
	Model             string        `json:"model"`
	StartTime         time.Time     `json:"startTime"`
	EndTime           time.Time     `json:"endTime"`
	Duration          time.Duration `json:"duration"`
	InputTokens       int64         `json:"inputTokens"`
	OutputTokens      int64         `json:"outputTokens"`
	CacheCreateTokens int64         `json:"cacheCreateTokens"`
	CacheReadTokens   int64         `json:"cacheReadTokens"`
	CostUSD           float64       `json:"costUsd"`
	Turns             int           `json:"turns"`
}

// DailyStat aggregates session records by calendar date.
type DailyStat struct {
	Date         string             `json:"date"` // "2026-02-15"
	Sessions     int                `json:"sessions"`
	TotalCost    float64            `json:"totalCost"`
	InputTokens  int64              `json:"inputTokens"`
	OutputTokens int64              `json:"outputTokens"`
	ByProject    map[string]float64 `json:"byProject"` // project alias -> cost
	ByModel      map[string]float64 `json:"byModel"`   // model name -> cost
	ByProfile    map[string]float64 `json:"byProfile"` // API profile name -> cost
}

// StatsCache is the on-disk cache of all scanned session data.
type StatsCache struct {
	LastScan   time.Time       `json:"lastScan"`
	Sessions   []SessionRecord `json:"sessions"`
	DailyStats []DailyStat     `json:"dailyStats"`
}

// Usage holds token counts extracted from a Claude assistant message.
type Usage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CacheCreateTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadTokens   int64 `json:"cache_read_input_tokens"`
}

// Summary represents aggregated statistics across a time period.
type Summary struct {
	TotalCost      float64       `json:"totalCost"`
	TotalSessions  int           `json:"totalSessions"`
	InputTokens    int64         `json:"inputTokens"`
	OutputTokens   int64         `json:"outputTokens"`
	CacheCreate    int64         `json:"cacheCreateTokens"`
	CacheRead      int64         `json:"cacheReadTokens"`
	TopProjects    []ProjectCost `json:"topProjects"`
	TopModels      []ModelCost   `json:"topModels"`
	TopProfiles    []ProfileCost `json:"topProfiles"`
	DailyBreakdown []DailyStat   `json:"dailyBreakdown"`
}

// ProjectCost represents cost aggregation for a single project.
type ProjectCost struct {
	Project string  `json:"project"`
	Cost    float64 `json:"cost"`
}

// ModelCost represents cost aggregation for a single model.
type ModelCost struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
}

// ProfileCost represents cost aggregation for a single API profile.
type ProfileCost struct {
	Profile string  `json:"profile"`
	Cost    float64 `json:"cost"`
}
