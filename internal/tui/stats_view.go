package tui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"codes/internal/stats"
)

// Stats-specific styles
var (
	statsHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	statsCostStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E8E4E0"))

	statsBarFilledStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	statsBarEmptyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#3A3F47"))

	statsDimStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	statsAccentStyle = lipgloss.NewStyle().
				Foreground(secondaryColor)
)

// loadStatsCmd creates a tea.Cmd that loads stats data asynchronously.
func loadStatsCmd(timeRange string) tea.Cmd {
	return func() tea.Msg {
		cache, err := stats.LoadCache()
		if err != nil {
			cache = &stats.StatsCache{}
		}

		cache, err = stats.RefreshIfNeeded(cache)
		if err != nil {
			return statsLoadedMsg{err: err}
		}

		var from, to time.Time
		switch timeRange {
		case "week":
			from, to = stats.ThisWeekRange()
		case "month":
			from, to = stats.ThisMonthRange()
		case "all":
			// zero values = no filter
		}

		daily := stats.Aggregate(cache.Sessions, from, to)
		return statsLoadedMsg{daily: daily, records: cache.Sessions}
	}
}

// updateStats handles key events in the Stats view.
func (m Model) updateStats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "w":
		m.statsRange = "week"
		m.statsLoading = true
		return m, loadStatsCmd("week")
	case "m":
		m.statsRange = "month"
		m.statsLoading = true
		return m, loadStatsCmd("month")
	case "a":
		m.statsRange = "all"
		m.statsLoading = true
		return m, loadStatsCmd("all")
	case "f":
		switch m.statsBreakdown {
		case "", "both":
			m.statsBreakdown = "project"
		case "project":
			m.statsBreakdown = "model"
		case "model":
			m.statsBreakdown = "profile"
		case "profile":
			m.statsBreakdown = "both"
		}
		return m, nil
	case "r":
		m.statsLoading = true
		return m, func() tea.Msg {
			cache, _ := stats.LoadCache()
			cache, err := stats.ForceRefresh(cache)
			if err != nil {
				return statsLoadedMsg{err: err}
			}
			var from, to time.Time
			switch m.statsRange {
			case "week":
				from, to = stats.ThisWeekRange()
			case "month":
				from, to = stats.ThisMonthRange()
			}
			daily := stats.Aggregate(cache.Sessions, from, to)
			return statsLoadedMsg{daily: daily, records: cache.Sessions}
		}
	}
	return m, nil
}

// renderStatsView renders the full stats panel.
func renderStatsView(daily []stats.DailyStat, records []stats.SessionRecord, timeRange, breakdown string, loading bool, width, height int) string {
	if loading {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(statsDimStyle.Render("Loading stats..."))
	}

	if breakdown == "" {
		breakdown = "both"
	}

	var b strings.Builder

	// Title with time range
	rangeLabel, dateRange := timeRangeLabel(timeRange)
	title := statsHeaderStyle.Render(fmt.Sprintf("  Usage %s", rangeLabel))
	if dateRange != "" {
		title += statsDimStyle.Render(fmt.Sprintf(" (%s)", dateRange))
	}
	b.WriteString(title)
	b.WriteString("\n\n")

	// Summary line
	totalCost := stats.TotalCost(daily)
	totalSessions := stats.TotalSessions(daily)
	var totalIn, totalOut int64
	for _, d := range daily {
		totalIn += d.InputTokens
		totalOut += d.OutputTokens
	}

	summary := fmt.Sprintf("  Total: %s         Sessions: %s",
		statsCostStyle.Render(formatCost(totalCost)),
		statsCostStyle.Render(fmt.Sprintf("%d", totalSessions)))
	b.WriteString(summary)
	b.WriteString("\n")

	tokens := fmt.Sprintf("  Tokens: %s in / %s out",
		formatTokens(totalIn),
		formatTokens(totalOut))
	b.WriteString(statsDimStyle.Render(tokens))
	b.WriteString("\n\n")

	barWidth := min(width-40, 30)
	if barWidth < 10 {
		barWidth = 10
	}

	// By Project breakdown
	if breakdown == "both" || breakdown == "project" {
		projectCosts := aggregateByProject(daily)
		if len(projectCosts) > 0 {
			b.WriteString(statsHeaderStyle.Render("  By Project:"))
			b.WriteString("\n")
			renderBreakdown(&b, projectCosts, totalCost, barWidth, 8)
			b.WriteString("\n")
		}
	}

	// By Model breakdown
	if breakdown == "both" || breakdown == "model" {
		modelCosts := aggregateByModel(daily)
		if len(modelCosts) > 0 {
			b.WriteString(statsHeaderStyle.Render("  By Model:"))
			b.WriteString("\n")
			renderBreakdown(&b, modelCosts, totalCost, barWidth, 8)
			b.WriteString("\n")
		}
	}

	// By Profile breakdown
	if breakdown == "both" || breakdown == "profile" {
		profileCosts := aggregateByProfile(daily)
		if len(profileCosts) > 0 {
			b.WriteString(statsHeaderStyle.Render("  By Profile:"))
			b.WriteString("\n")
			renderBreakdown(&b, profileCosts, totalCost, barWidth, 8)
			b.WriteString("\n")
		}
	}

	// Daily trend chart
	if len(daily) > 1 {
		b.WriteString(statsHeaderStyle.Render("  Daily Trend:"))
		b.WriteString("\n")
		renderDailyChart(&b, daily, width-6, 6)
	}

	// Help footer
	b.WriteString("\n")
	breakdownLabel := map[string]string{"both": "all", "project": "project", "model": "model", "profile": "profile"}[breakdown]
	help := fmt.Sprintf("  w:week  m:month  a:all  f:group(%s)  r:refresh", breakdownLabel)
	b.WriteString(statsDimStyle.Render(help))

	return b.String()
}

type costEntry struct {
	name string
	cost float64
}

func aggregateByProject(daily []stats.DailyStat) []costEntry {
	totals := make(map[string]float64)
	for _, d := range daily {
		for proj, cost := range d.ByProject {
			totals[proj] += cost
		}
	}
	return sortedEntries(totals)
}

func aggregateByModel(daily []stats.DailyStat) []costEntry {
	totals := make(map[string]float64)
	for _, d := range daily {
		for model, cost := range d.ByModel {
			// Shorten model names for display
			totals[shortenModel(model)] += cost
		}
	}
	return sortedEntries(totals)
}

func aggregateByProfile(daily []stats.DailyStat) []costEntry {
	totals := make(map[string]float64)
	for _, d := range daily {
		for profile, cost := range d.ByProfile {
			totals[profile] += cost
		}
	}
	return sortedEntries(totals)
}

func sortedEntries(m map[string]float64) []costEntry {
	entries := make([]costEntry, 0, len(m))
	for name, cost := range m {
		entries = append(entries, costEntry{name: name, cost: cost})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].cost > entries[j].cost
	})
	// Limit to top 6 + "others"
	if len(entries) > 6 {
		otherCost := 0.0
		for _, e := range entries[6:] {
			otherCost += e.cost
		}
		entries = append(entries[:6], costEntry{name: "others", cost: otherCost})
	}
	return entries
}

func renderBreakdown(b *strings.Builder, entries []costEntry, total float64, barWidth, maxEntries int) {
	if len(entries) > maxEntries {
		entries = entries[:maxEntries]
	}

	// Find max name length for alignment
	maxNameLen := 0
	for _, e := range entries {
		if len(e.name) > maxNameLen {
			maxNameLen = len(e.name)
		}
	}
	if maxNameLen > 20 {
		maxNameLen = 20
	}

	for _, e := range entries {
		pct := 0.0
		if total > 0 {
			pct = e.cost / total * 100
		}
		filled := int(math.Round(float64(barWidth) * e.cost / total))
		if filled < 0 {
			filled = 0
		}
		if filled > barWidth {
			filled = barWidth
		}
		empty := barWidth - filled

		bar := statsBarFilledStyle.Render(strings.Repeat("█", filled)) +
			statsBarEmptyStyle.Render(strings.Repeat("░", empty))

		name := e.name
		if len(name) > maxNameLen {
			name = name[:maxNameLen]
		}

		line := fmt.Sprintf("  %s  %-*s  %7s  %s",
			bar,
			maxNameLen, name,
			formatCost(e.cost),
			statsDimStyle.Render(fmt.Sprintf("(%4.1f%%)", pct)))
		b.WriteString(line)
		b.WriteString("\n")
	}
}

func renderDailyChart(b *strings.Builder, daily []stats.DailyStat, width, maxHeight int) {
	if len(daily) == 0 {
		return
	}

	// Find max cost for scaling
	maxCost := 0.0
	for _, d := range daily {
		if d.TotalCost > maxCost {
			maxCost = d.TotalCost
		}
	}
	if maxCost == 0 {
		return
	}

	// Calculate bar width
	yLabelWidth := len(formatCost(maxCost)) + 1 // e.g. "$12.34 "
	barSpacing := 2                              // each bar + gap
	maxBars := (width - yLabelWidth - 2) / barSpacing
	startIdx := 0
	if len(daily) > maxBars {
		startIdx = len(daily) - maxBars
	}
	visibleDays := daily[startIdx:]

	// Build vertical bars (top to bottom) with Y-axis labels
	for row := maxHeight; row >= 1; row-- {
		threshold := float64(row) / float64(maxHeight)

		// Y-axis label: show at top, middle, bottom
		label := strings.Repeat(" ", yLabelWidth)
		if row == maxHeight {
			label = fmt.Sprintf("%*s ", yLabelWidth-1, formatCost(maxCost))
		} else if row == maxHeight/2 {
			label = fmt.Sprintf("%*s ", yLabelWidth-1, formatCost(maxCost/2))
		} else if row == 1 {
			label = fmt.Sprintf("%*s ", yLabelWidth-1, "$0")
		}
		line := statsDimStyle.Render(label)

		for _, d := range visibleDays {
			ratio := d.TotalCost / maxCost
			if ratio >= threshold {
				line += statsBarFilledStyle.Render("█") + " "
			} else {
				line += "  "
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Date labels (first, middle, last)
	if len(visibleDays) > 0 {
		first := shortDate(visibleDays[0].Date)
		last := shortDate(visibleDays[len(visibleDays)-1].Date)

		labelWidth := len(visibleDays) * barSpacing
		gap := labelWidth - len(first) - len(last)
		if gap < 1 {
			gap = 1
		}
		b.WriteString(strings.Repeat(" ", yLabelWidth) + statsDimStyle.Render(first+strings.Repeat(" ", gap)+last))
		b.WriteString("\n")
	}
}

// Helper functions

func timeRangeLabel(r string) (string, string) {
	now := time.Now()
	switch r {
	case "week":
		from, _ := stats.ThisWeekRange()
		return "This Week", fmt.Sprintf("%s — %s",
			from.Format("Jan 2"), now.Format("Jan 2"))
	case "month":
		return "This Month", now.Format("January 2006")
	case "all":
		return "All Time", ""
	default:
		return "This Week", ""
	}
}

func formatCost(cost float64) string {
	if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	}
	return fmt.Sprintf("$%.2f", cost)
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func shortenModel(model string) string {
	// Strip common prefixes for cleaner display
	replacements := map[string]string{
		"claude-opus-4-":     "opus-4-",
		"claude-sonnet-4-":   "sonnet-4-",
		"claude-haiku-3-":    "haiku-3-",
		"claude-3-5-sonnet-": "sonnet-3.5-",
		"claude-3-5-haiku-":  "haiku-3.5-",
		"claude-3-opus-":     "opus-3-",
	}
	for prefix, short := range replacements {
		if strings.HasPrefix(model, prefix) {
			return short + model[len(prefix):]
		}
	}
	return model
}

func shortDate(dateStr string) string {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return dateStr
	}
	return t.Format("Jan 2")
}
