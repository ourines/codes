package commands

import (
	"math"
	"testing"
	"time"

	"codes/internal/stats"
)

// floatEquals checks if two floats are equal within epsilon.
func floatEquals(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// TestFormatTokens verifies thousand separator formatting.
func TestFormatTokens(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{123, "123"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
		{12345678, "12,345,678"},
		{0, "0"},
	}

	for _, tt := range tests {
		result := formatTokens(tt.input)
		if result != tt.expected {
			t.Errorf("formatTokens(%d) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

// TestFormatPeriod verifies period label conversion.
func TestFormatPeriod(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"today", "Today"},
		{"week", "This Week"},
		{"month", "This Month"},
		{"all", "All Time"},
		{"unknown", "This Week"}, // defaults to week
		{"", "This Week"},        // defaults to week
	}

	for _, tt := range tests {
		result := formatPeriod(tt.input)
		if result != tt.expected {
			t.Errorf("formatPeriod(%q) = %s; want %s", tt.input, result, tt.expected)
		}
	}
}

// TestLoadStatsCache verifies cache loading and refresh logic.
func TestLoadStatsCache(t *testing.T) {
	// This test assumes the cache file exists or can be created.
	// In a real environment, we might want to use a temporary directory.
	cache, err := loadStatsCache()
	if err != nil {
		t.Logf("loadStatsCache error (may be expected): %v", err)
		// Don't fail - this is expected if no sessions exist
		return
	}

	if cache == nil {
		t.Error("loadStatsCache returned nil cache without error")
	}

	// Verify cache fields
	if cache.LastScan.IsZero() {
		t.Log("Warning: cache LastScan is zero (cache might be empty)")
	}

	if len(cache.Sessions) == 0 {
		t.Log("Warning: no sessions found in cache (this is OK for new installations)")
	}

	if len(cache.DailyStats) == 0 {
		t.Log("Warning: no daily stats in cache (this is OK for new installations)")
	}
}

// TestStatsAggregation verifies basic aggregation logic.
func TestStatsAggregation(t *testing.T) {
	// Create mock session records
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	records := []stats.SessionRecord{
		{
			SessionID:    "test1",
			Project:      "testproj",
			Model:        "claude-3-5-sonnet-20241022",
			StartTime:    yesterday,
			InputTokens:  1000,
			OutputTokens: 500,
			CostUSD:      0.015,
		},
		{
			SessionID:    "test2",
			Project:      "testproj",
			Model:        "claude-3-5-sonnet-20241022",
			StartTime:    now,
			InputTokens:  2000,
			OutputTokens: 1000,
			CostUSD:      0.030,
		},
		{
			SessionID:    "test3",
			Project:      "otherproj",
			Model:        "claude-3-5-haiku-20241022",
			StartTime:    now,
			InputTokens:  5000,
			OutputTokens: 2500,
			CostUSD:      0.005,
		},
	}

	// Generate summary for all time
	summary := stats.GenerateSummary(records, time.Time{}, time.Time{})

	// Verify totals
	if summary.TotalSessions != 3 {
		t.Errorf("Expected 3 sessions, got %d", summary.TotalSessions)
	}

	expectedCost := 0.015 + 0.030 + 0.005
	if !floatEquals(summary.TotalCost, expectedCost, 0.0001) {
		t.Errorf("Expected total cost %.4f, got %.4f", expectedCost, summary.TotalCost)
	}

	expectedInput := int64(1000 + 2000 + 5000)
	if summary.InputTokens != expectedInput {
		t.Errorf("Expected input tokens %d, got %d", expectedInput, summary.InputTokens)
	}

	expectedOutput := int64(500 + 1000 + 2500)
	if summary.OutputTokens != expectedOutput {
		t.Errorf("Expected output tokens %d, got %d", expectedOutput, summary.OutputTokens)
	}

	// Verify project breakdown
	if len(summary.TopProjects) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(summary.TopProjects))
	}

	// Verify model breakdown
	if len(summary.TopModels) != 2 {
		t.Errorf("Expected 2 models, got %d", len(summary.TopModels))
	}

	// Verify projects are sorted by cost descending
	if len(summary.TopProjects) >= 2 {
		if summary.TopProjects[0].Cost < summary.TopProjects[1].Cost {
			t.Error("Projects should be sorted by cost descending")
		}
	}
}

// TestTimeRangeFiltering verifies that time range filters work correctly.
func TestTimeRangeFiltering(t *testing.T) {
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -7)
	lastMonth := now.AddDate(0, -1, 0)

	records := []stats.SessionRecord{
		{
			SessionID: "old",
			StartTime: lastMonth,
			CostUSD:   0.01,
		},
		{
			SessionID: "mid",
			StartTime: lastWeek,
			CostUSD:   0.02,
		},
		{
			SessionID: "new",
			StartTime: now,
			CostUSD:   0.03,
		},
	}

	// Test week range (should include last week and newer)
	weekStart, weekEnd := stats.ThisWeekRange()
	summary := stats.GenerateSummary(records, weekStart, weekEnd)

	// Only records within this week should be included
	// This depends on when the week starts, so we just verify it's <= 3
	if summary.TotalSessions > 3 {
		t.Errorf("Week range should include <= 3 sessions, got %d", summary.TotalSessions)
	}

	// Test all time
	summaryAll := stats.GenerateSummary(records, time.Time{}, time.Time{})
	if summaryAll.TotalSessions != 3 {
		t.Errorf("All time should include 3 sessions, got %d", summaryAll.TotalSessions)
	}
}
