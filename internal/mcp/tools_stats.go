package mcpserver

import (
	"context"
	"fmt"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/stats"
)

// registerStatsTools registers stats-related MCP tools.
func registerStatsTools(server *mcpsdk.Server) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "stats_summary",
		Description: "Get Claude usage cost summary for a time period (today, week, month, all)",
	}, statsSummaryHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "stats_by_project",
		Description: "Get cost breakdown by project. Optionally filter to a specific project.",
	}, statsByProjectHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "stats_by_model",
		Description: "Get cost breakdown by Claude model",
	}, statsByModelHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "stats_refresh",
		Description: "Force a full rescan of Claude session files and rebuild the stats cache",
	}, statsRefreshHandler)
}

// stats_summary types

type statsSummaryInput struct {
	Period string `json:"period" jsonschema:"Time period: today, week, month, all (default: week)"`
}

type statsSummaryOutput struct {
	Period        string                `json:"period"`
	TotalCost     float64               `json:"totalCost"`
	TotalSessions int                   `json:"totalSessions"`
	InputTokens   int64                 `json:"inputTokens"`
	OutputTokens  int64                 `json:"outputTokens"`
	CacheCreate   int64                 `json:"cacheCreate"`
	CacheRead     int64                 `json:"cacheRead"`
	TopProjects   []stats.ProjectCost   `json:"topProjects"`
	TopModels     []stats.ModelCost     `json:"topModels"`
}

func statsSummaryHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input statsSummaryInput) (*mcpsdk.CallToolResult, statsSummaryOutput, error) {
	period := input.Period
	if period == "" {
		period = "week"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		return nil, statsSummaryOutput{}, fmt.Errorf("failed to load stats cache: %w", err)
	}

	cache, _ = stats.RefreshIfNeeded(cache)

	from, to := parseTimeRange(period)
	summary := stats.GenerateSummary(cache.Sessions, from, to)

	output := statsSummaryOutput{
		Period:        period,
		TotalCost:     summary.TotalCost,
		TotalSessions: summary.TotalSessions,
		InputTokens:   summary.InputTokens,
		OutputTokens:  summary.OutputTokens,
		CacheCreate:   summary.CacheCreate,
		CacheRead:     summary.CacheRead,
		TopProjects:   summary.TopProjects,
		TopModels:     summary.TopModels,
	}

	return nil, output, nil
}

// stats_by_project types

type statsByProjectInput struct {
	Project string `json:"project" jsonschema:"Optional project name to filter by"`
	Period  string `json:"period" jsonschema:"Time period: today, week, month, all (default: all)"`
}

type statsByProjectOutput struct {
	Projects []stats.ProjectCost `json:"projects"`
}

func statsByProjectHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input statsByProjectInput) (*mcpsdk.CallToolResult, statsByProjectOutput, error) {
	period := input.Period
	if period == "" {
		period = "all"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		return nil, statsByProjectOutput{}, fmt.Errorf("failed to load stats cache: %w", err)
	}

	cache, _ = stats.RefreshIfNeeded(cache)

	from, to := parseTimeRange(period)
	summary := stats.GenerateSummary(cache.Sessions, from, to)

	if input.Project != "" {
		// Filter for specific project
		var found *stats.ProjectCost
		for _, p := range summary.TopProjects {
			if p.Project == input.Project {
				found = &p
				break
			}
		}
		if found == nil {
			return nil, statsByProjectOutput{}, fmt.Errorf("project not found: %s", input.Project)
		}
		return nil, statsByProjectOutput{Projects: []stats.ProjectCost{*found}}, nil
	}

	return nil, statsByProjectOutput{Projects: summary.TopProjects}, nil
}

// stats_by_model types

type statsByModelInput struct {
	Period string `json:"period" jsonschema:"Time period: today, week, month, all (default: all)"`
}

type statsByModelOutput struct {
	Models []stats.ModelCost `json:"models"`
}

func statsByModelHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input statsByModelInput) (*mcpsdk.CallToolResult, statsByModelOutput, error) {
	period := input.Period
	if period == "" {
		period = "all"
	}

	cache, err := stats.LoadCache()
	if err != nil {
		return nil, statsByModelOutput{}, fmt.Errorf("failed to load stats cache: %w", err)
	}

	cache, _ = stats.RefreshIfNeeded(cache)

	from, to := parseTimeRange(period)
	summary := stats.GenerateSummary(cache.Sessions, from, to)

	return nil, statsByModelOutput{Models: summary.TopModels}, nil
}

// stats_refresh types

type statsRefreshInput struct{}

type statsRefreshOutput struct {
	Success       bool      `json:"success"`
	LastScan      time.Time `json:"lastScan"`
	TotalSessions int       `json:"totalSessions"`
	Message       string    `json:"message"`
}

func statsRefreshHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input statsRefreshInput) (*mcpsdk.CallToolResult, statsRefreshOutput, error) {
	cache, err := stats.LoadCache()
	if err != nil {
		return nil, statsRefreshOutput{}, fmt.Errorf("failed to load cache: %w", err)
	}

	cache, err = stats.ForceRefresh(cache)
	if err != nil {
		return nil, statsRefreshOutput{}, fmt.Errorf("failed to refresh cache: %w", err)
	}

	output := statsRefreshOutput{
		Success:       true,
		LastScan:      cache.LastScan,
		TotalSessions: len(cache.Sessions),
		Message:       fmt.Sprintf("Successfully scanned %d sessions", len(cache.Sessions)),
	}

	return nil, output, nil
}

// parseTimeRange converts a period string to time range.
func parseTimeRange(period string) (time.Time, time.Time) {
	switch period {
	case "today":
		now := time.Now()
		from := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return from, now
	case "week":
		return stats.ThisWeekRange()
	case "month":
		return stats.ThisMonthRange()
	case "all":
		return time.Time{}, time.Time{}
	default:
		return stats.ThisWeekRange()
	}
}
