package stats

import (
	"sort"
	"time"
)

// Aggregate groups session records into daily statistics.
// If from/to are zero values, all records are included.
func Aggregate(records []SessionRecord, from, to time.Time) []DailyStat {
	dailyMap := make(map[string]*DailyStat)

	for _, r := range records {
		// Time range filter
		if !from.IsZero() && r.StartTime.Before(from) {
			continue
		}
		if !to.IsZero() && r.StartTime.After(to) {
			continue
		}

		date := r.StartTime.Format("2006-01-02")
		ds, ok := dailyMap[date]
		if !ok {
			ds = &DailyStat{
				Date:      date,
				ByProject: make(map[string]float64),
				ByModel:   make(map[string]float64),
			}
			dailyMap[date] = ds
		}

		ds.Sessions++
		ds.TotalCost += r.CostUSD
		ds.InputTokens += r.InputTokens
		ds.OutputTokens += r.OutputTokens
		ds.ByProject[r.Project] += r.CostUSD
		ds.ByModel[r.Model] += r.CostUSD
	}

	// Convert map to sorted slice
	result := make([]DailyStat, 0, len(dailyMap))
	for _, ds := range dailyMap {
		result = append(result, *ds)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result
}

// TimeRange helpers for common filter periods.

// ThisWeekRange returns the start of the current ISO week (Monday) and now.
func ThisWeekRange() (time.Time, time.Time) {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday = 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	start := time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, now.Location())
	return start, now
}

// ThisMonthRange returns the start of the current month and now.
func ThisMonthRange() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	return start, now
}

// Last7DaysRange returns 7 days ago and now.
func Last7DaysRange() (time.Time, time.Time) {
	now := time.Now()
	return now.AddDate(0, 0, -7), now
}

// Last30DaysRange returns 30 days ago and now.
func Last30DaysRange() (time.Time, time.Time) {
	now := time.Now()
	return now.AddDate(0, 0, -30), now
}

// TotalCost sums costs across a slice of daily stats.
func TotalCost(stats []DailyStat) float64 {
	var total float64
	for _, s := range stats {
		total += s.TotalCost
	}
	return total
}

// TotalSessions sums session counts across a slice of daily stats.
func TotalSessions(stats []DailyStat) int {
	var total int
	for _, s := range stats {
		total += s.Sessions
	}
	return total
}

// TotalTokens sums input and output tokens across a slice of daily stats.
func TotalTokens(stats []DailyStat) (input, output int64) {
	for _, s := range stats {
		input += s.InputTokens
		output += s.OutputTokens
	}
	return
}

// ProjectBreakdown aggregates costs by project across all daily stats.
// Returns a map of project name to total cost, sorted by cost descending.
func ProjectBreakdown(stats []DailyStat) []ProjectCost {
	totals := make(map[string]float64)
	for _, s := range stats {
		for proj, cost := range s.ByProject {
			totals[proj] += cost
		}
	}

	result := make([]ProjectCost, 0, len(totals))
	for proj, cost := range totals {
		result = append(result, ProjectCost{Project: proj, Cost: cost})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})

	return result
}

// ModelBreakdown aggregates costs by model across all daily stats.
// Returns a map of model name to total cost, sorted by cost descending.
func ModelBreakdown(stats []DailyStat) []ModelCost {
	totals := make(map[string]float64)
	for _, s := range stats {
		for model, cost := range s.ByModel {
			totals[model] += cost
		}
	}

	result := make([]ModelCost, 0, len(totals))
	for model, cost := range totals {
		result = append(result, ModelCost{Model: model, Cost: cost})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Cost > result[j].Cost
	})

	return result
}

// GenerateSummary creates a comprehensive summary from session records.
func GenerateSummary(records []SessionRecord, from, to time.Time) Summary {
	dailyStats := Aggregate(records, from, to)

	// Calculate total cache tokens from records
	var cacheCreate, cacheRead int64
	for _, r := range records {
		// Apply time filter
		if !from.IsZero() && r.StartTime.Before(from) {
			continue
		}
		if !to.IsZero() && r.StartTime.After(to) {
			continue
		}
		cacheCreate += r.CacheCreateTokens
		cacheRead += r.CacheReadTokens
	}

	input, output := TotalTokens(dailyStats)

	return Summary{
		TotalCost:      TotalCost(dailyStats),
		TotalSessions:  TotalSessions(dailyStats),
		InputTokens:    input,
		OutputTokens:   output,
		CacheCreate:    cacheCreate,
		CacheRead:      cacheRead,
		TopProjects:    ProjectBreakdown(dailyStats),
		TopModels:      ModelBreakdown(dailyStats),
		DailyBreakdown: dailyStats,
	}
}
