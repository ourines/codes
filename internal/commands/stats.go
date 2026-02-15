package commands

import (
	"fmt"
	"strings"
	"time"

	"codes/internal/output"
	"codes/internal/stats"
	"codes/internal/ui"
)

// RunStatsSummary displays overall statistics for a given period.
func RunStatsSummary(period string) {
	cache, err := loadStatsCache()
	if err != nil {
		handleStatsError(err)
		return
	}

	// Determine time range
	var from, to time.Time
	switch period {
	case "today":
		now := time.Now()
		from = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		to = now
	case "week":
		from, to = stats.ThisWeekRange()
	case "month":
		from, to = stats.ThisMonthRange()
	case "all":
		from, to = time.Time{}, time.Time{}
	default:
		from, to = stats.ThisWeekRange()
	}

	summary := stats.GenerateSummary(cache.Sessions, from, to)

	// Output
	if output.JSONMode {
		output.Print(map[string]interface{}{
			"period":        period,
			"totalCost":     summary.TotalCost,
			"totalSessions": summary.TotalSessions,
			"inputTokens":   summary.InputTokens,
			"outputTokens":  summary.OutputTokens,
			"cacheCreate":   summary.CacheCreate,
			"cacheRead":     summary.CacheRead,
			"topProjects":   summary.TopProjects,
			"topModels":     summary.TopModels,
		}, func() {})
	} else {
		printSummaryText(summary, period)
	}
}

// RunStatsProject displays cost breakdown by project.
func RunStatsProject(projectFilter string) {
	cache, err := loadStatsCache()
	if err != nil {
		handleStatsError(err)
		return
	}

	summary := stats.GenerateSummary(cache.Sessions, time.Time{}, time.Time{})

	if output.JSONMode {
		if projectFilter != "" {
			// Find specific project
			for _, p := range summary.TopProjects {
				if p.Project == projectFilter {
					output.Print(p, func() {})
					return
				}
			}
			output.PrintError(fmt.Errorf("project not found: %s", projectFilter))
		} else {
			output.Print(summary.TopProjects, func() {})
		}
	} else {
		printProjectBreakdown(summary.TopProjects, projectFilter)
	}
}

// RunStatsModel displays cost breakdown by model.
func RunStatsModel() {
	cache, err := loadStatsCache()
	if err != nil {
		handleStatsError(err)
		return
	}

	summary := stats.GenerateSummary(cache.Sessions, time.Time{}, time.Time{})

	if output.JSONMode {
		output.Print(summary.TopModels, func() {})
	} else {
		printModelBreakdown(summary.TopModels, summary.TotalCost)
	}
}

// RunStatsRefresh forces a full cache refresh.
func RunStatsRefresh() {
	cache, err := stats.LoadCache()
	if err != nil {
		handleStatsError(err)
		return
	}

	ui.ShowInfo("Scanning Claude session files...")
	cache, err = stats.ForceRefresh(cache)
	if err != nil {
		handleStatsError(err)
		return
	}

	if output.JSONMode {
		output.Print(map[string]interface{}{
			"success":      true,
			"sessionsFound": len(cache.Sessions),
			"lastScan":     cache.LastScan,
		}, func() {})
	} else {
		ui.ShowSuccess("Cache refreshed successfully")
		fmt.Printf("  Found %d sessions\n", len(cache.Sessions))
		fmt.Printf("  Last scan: %s\n", cache.LastScan.Format(time.RFC3339))
	}
}

// --- Helper functions ---

// loadStatsCache loads and refreshes the cache if needed.
func loadStatsCache() (*stats.StatsCache, error) {
	cache, err := stats.LoadCache()
	if err != nil {
		return nil, fmt.Errorf("load stats cache: %w", err)
	}

	cache, err = stats.RefreshIfNeeded(cache)
	if err != nil {
		// Log warning but continue with cached data
		ui.ShowWarning("Failed to refresh stats: %v", err)
	}

	return cache, nil
}

// handleStatsError handles stats errors in JSON or text mode.
func handleStatsError(err error) {
	if output.JSONMode {
		output.PrintError(err)
	} else {
		ui.ShowError("Stats error", err)
	}
}

// printSummaryText outputs summary in human-readable text format.
func printSummaryText(summary stats.Summary, period string) {
	title := fmt.Sprintf("Claude Usage Summary (%s)", formatPeriod(period))
	ui.ShowHeader(title)
	fmt.Println()

	fmt.Printf("  Total Cost:     $%.4f\n", summary.TotalCost)
	fmt.Printf("  Sessions:       %d\n", summary.TotalSessions)
	fmt.Printf("  Input Tokens:   %s\n", formatTokens(summary.InputTokens))
	fmt.Printf("  Output Tokens:  %s\n", formatTokens(summary.OutputTokens))
	if summary.CacheCreate > 0 || summary.CacheRead > 0 {
		fmt.Printf("  Cache Create:   %s\n", formatTokens(summary.CacheCreate))
		fmt.Printf("  Cache Read:     %s\n", formatTokens(summary.CacheRead))
	}
	fmt.Println()

	// Top projects (limit to 5)
	if len(summary.TopProjects) > 0 {
		fmt.Println("  Top Projects:")
		for i, p := range summary.TopProjects {
			if i >= 5 {
				break
			}
			pct := (p.Cost / summary.TotalCost) * 100
			fmt.Printf("    %2d. %-30s $%.4f (%.1f%%)\n", i+1, p.Project, p.Cost, pct)
		}
		fmt.Println()
	}

	// Top models (limit to 5)
	if len(summary.TopModels) > 0 {
		fmt.Println("  Top Models:")
		for i, m := range summary.TopModels {
			if i >= 5 {
				break
			}
			pct := (m.Cost / summary.TotalCost) * 100
			fmt.Printf("    %2d. %-30s $%.4f (%.1f%%)\n", i+1, m.Model, m.Cost, pct)
		}
		fmt.Println()
	}

	ui.ShowInfo("Use 'codes stats project' or 'codes stats model' for detailed breakdowns")
}

// printProjectBreakdown outputs project costs in text format.
func printProjectBreakdown(projects []stats.ProjectCost, filter string) {
	ui.ShowHeader("Cost Breakdown by Project")
	fmt.Println()

	if filter != "" {
		// Show specific project
		for _, p := range projects {
			if p.Project == filter {
				fmt.Printf("  Project: %s\n", p.Project)
				fmt.Printf("  Cost:    $%.4f\n", p.Cost)
				return
			}
		}
		ui.ShowError(fmt.Sprintf("Project not found: %s", filter), nil)
		return
	}

	// Show all projects
	var total float64
	for _, p := range projects {
		total += p.Cost
	}

	for i, p := range projects {
		pct := (p.Cost / total) * 100
		fmt.Printf("  %2d. %-40s $%.4f (%.1f%%)\n", i+1, p.Project, p.Cost, pct)
	}
	fmt.Println()
	fmt.Printf("  Total: $%.4f\n", total)
}

// printModelBreakdown outputs model costs in text format.
func printModelBreakdown(models []stats.ModelCost, total float64) {
	ui.ShowHeader("Cost Breakdown by Model")
	fmt.Println()

	for i, m := range models {
		pct := (m.Cost / total) * 100
		fmt.Printf("  %2d. %-40s $%.4f (%.1f%%)\n", i+1, m.Model, m.Cost, pct)
	}
	fmt.Println()
	fmt.Printf("  Total: $%.4f\n", total)
}

// formatPeriod converts a period code to a human-readable label.
func formatPeriod(p string) string {
	switch p {
	case "today":
		return "Today"
	case "week":
		return "This Week"
	case "month":
		return "This Month"
	case "all":
		return "All Time"
	default:
		return "This Week"
	}
}

// formatTokens formats large token counts with thousand separators.
func formatTokens(n int64) string {
	s := fmt.Sprintf("%d", n)
	parts := []string{}
	for i := len(s); i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		parts = append([]string{s[start:i]}, parts...)
	}
	return strings.Join(parts, ",")
}
