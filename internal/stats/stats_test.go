package stats

import (
	"math"
	"testing"
	"time"
)

func TestCalculateCost_Opus(t *testing.T) {
	usage := Usage{
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
		CacheCreateTokens: 0,
		CacheReadTokens:   0,
	}
	cost := CalculateCost("claude-opus-4-6", usage)
	// input: 1M * $15/1M = $15, output: 1M * $75/1M = $75
	expected := 90.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("opus cost = %f, want %f", cost, expected)
	}
}

func TestCalculateCost_Sonnet(t *testing.T) {
	usage := Usage{
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
		CacheCreateTokens: 0,
		CacheReadTokens:   0,
	}
	cost := CalculateCost("claude-sonnet-4-5-20250929", usage)
	// input: 1M * $3/1M = $3, output: 1M * $15/1M = $15
	expected := 18.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("sonnet cost = %f, want %f", cost, expected)
	}
}

func TestCalculateCost_Haiku(t *testing.T) {
	usage := Usage{
		InputTokens:       1_000_000,
		OutputTokens:      1_000_000,
		CacheCreateTokens: 0,
		CacheReadTokens:   0,
	}
	cost := CalculateCost("claude-haiku-3-5-20241022", usage)
	// input: 1M * $0.80/1M = $0.80, output: 1M * $4/1M = $4
	expected := 4.80
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("haiku cost = %f, want %f", cost, expected)
	}
}

func TestCalculateCost_WithCache(t *testing.T) {
	usage := Usage{
		InputTokens:       500_000,
		OutputTokens:      100_000,
		CacheCreateTokens: 200_000,
		CacheReadTokens:   300_000,
	}
	cost := CalculateCost("claude-sonnet-4-5-20250929", usage)
	// input: 500K * $3/1M = $1.50
	// output: 100K * $15/1M = $1.50
	// cache write: 200K * ($3 * 0.25)/1M = 200K * $0.75/1M = $0.15
	// cache read: 300K * ($3 * 0.10)/1M = 300K * $0.30/1M = $0.09
	expected := 1.50 + 1.50 + 0.15 + 0.09
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("sonnet+cache cost = %f, want %f", cost, expected)
	}
}

func TestCalculateCost_UnknownModel(t *testing.T) {
	usage := Usage{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	}
	cost := CalculateCost("claude-unknown-model", usage)
	// Falls back to sonnet pricing
	expected := 18.0
	if math.Abs(cost-expected) > 0.001 {
		t.Errorf("unknown model cost = %f, want %f", cost, expected)
	}
}

func TestProjectPathFromDir(t *testing.T) {
	tests := []struct {
		dir  string
		want string
	}{
		{"-Users-ourines-Projects-codes", "/Users/ourines/Projects/codes"},
		{"-home-user-my-project", "/home/user/my/project"},
		{"-tmp-test", "/tmp/test"},
	}
	for _, tt := range tests {
		got := projectPathFromDir(tt.dir)
		if got != tt.want {
			t.Errorf("projectPathFromDir(%q) = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestAggregate_BasicGrouping(t *testing.T) {
	records := []SessionRecord{
		{
			SessionID: "s1",
			Project:   "codes",
			Model:     "claude-sonnet-4-5-20250929",
			StartTime: time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC),
			CostUSD:   1.50,
			InputTokens: 100_000,
			OutputTokens: 50_000,
		},
		{
			SessionID: "s2",
			Project:   "codes",
			Model:     "claude-opus-4-6",
			StartTime: time.Date(2026, 2, 15, 14, 0, 0, 0, time.UTC),
			CostUSD:   5.00,
			InputTokens: 200_000,
			OutputTokens: 100_000,
		},
		{
			SessionID: "s3",
			Project:   "other",
			Model:     "claude-sonnet-4-5-20250929",
			StartTime: time.Date(2026, 2, 16, 9, 0, 0, 0, time.UTC),
			CostUSD:   2.00,
			InputTokens: 150_000,
			OutputTokens: 75_000,
		},
	}

	stats := Aggregate(records, time.Time{}, time.Time{})

	if len(stats) != 2 {
		t.Fatalf("expected 2 daily stats, got %d", len(stats))
	}

	// First day
	if stats[0].Date != "2026-02-15" {
		t.Errorf("day 1 date = %s, want 2026-02-15", stats[0].Date)
	}
	if stats[0].Sessions != 2 {
		t.Errorf("day 1 sessions = %d, want 2", stats[0].Sessions)
	}
	if math.Abs(stats[0].TotalCost-6.50) > 0.001 {
		t.Errorf("day 1 cost = %f, want 6.50", stats[0].TotalCost)
	}

	// Second day
	if stats[1].Date != "2026-02-16" {
		t.Errorf("day 2 date = %s, want 2026-02-16", stats[1].Date)
	}
	if stats[1].Sessions != 1 {
		t.Errorf("day 2 sessions = %d, want 1", stats[1].Sessions)
	}
}

func TestAggregate_TimeFilter(t *testing.T) {
	records := []SessionRecord{
		{
			SessionID: "s1",
			StartTime: time.Date(2026, 2, 10, 10, 0, 0, 0, time.UTC),
			CostUSD:   1.00,
		},
		{
			SessionID: "s2",
			StartTime: time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC),
			CostUSD:   2.00,
		},
	}

	from := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	stats := Aggregate(records, from, time.Time{})

	if len(stats) != 1 {
		t.Fatalf("expected 1 daily stat after filter, got %d", len(stats))
	}
	if math.Abs(stats[0].TotalCost-2.00) > 0.001 {
		t.Errorf("filtered cost = %f, want 2.00", stats[0].TotalCost)
	}
}

func TestLookupPricing(t *testing.T) {
	tests := []struct {
		model string
		input float64
	}{
		{"claude-opus-4-6", 15.0},
		{"claude-opus-4-20250115", 15.0},
		{"claude-sonnet-4-5-20250929", 3.0},
		{"claude-sonnet-4-20250514", 3.0},
		{"claude-haiku-3-5-20241022", 0.80},
		{"totally-unknown", 3.0}, // default = sonnet
	}
	for _, tt := range tests {
		p := lookupPricing(tt.model)
		if math.Abs(p.InputPerMillion-tt.input) > 0.001 {
			t.Errorf("lookupPricing(%q).InputPerMillion = %f, want %f", tt.model, p.InputPerMillion, tt.input)
		}
	}
}

func TestTotalTokens(t *testing.T) {
	stats := []DailyStat{
		{InputTokens: 100_000, OutputTokens: 50_000},
		{InputTokens: 200_000, OutputTokens: 75_000},
	}
	input, output := TotalTokens(stats)
	if input != 300_000 {
		t.Errorf("TotalTokens input = %d, want 300000", input)
	}
	if output != 125_000 {
		t.Errorf("TotalTokens output = %d, want 125000", output)
	}
}

func TestProjectBreakdown(t *testing.T) {
	stats := []DailyStat{
		{
			ByProject: map[string]float64{
				"codes":  5.00,
				"conduit": 3.00,
			},
		},
		{
			ByProject: map[string]float64{
				"codes":  2.00,
				"other": 1.00,
			},
		},
	}

	breakdown := ProjectBreakdown(stats)
	if len(breakdown) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(breakdown))
	}

	// Should be sorted by cost descending
	if breakdown[0].Project != "codes" || math.Abs(breakdown[0].Cost-7.00) > 0.001 {
		t.Errorf("top project = %s ($%.2f), want codes ($7.00)", breakdown[0].Project, breakdown[0].Cost)
	}
	if breakdown[1].Project != "conduit" || math.Abs(breakdown[1].Cost-3.00) > 0.001 {
		t.Errorf("2nd project = %s ($%.2f), want conduit ($3.00)", breakdown[1].Project, breakdown[1].Cost)
	}
}

func TestModelBreakdown(t *testing.T) {
	stats := []DailyStat{
		{
			ByModel: map[string]float64{
				"claude-opus-4-6":   10.00,
				"claude-sonnet-4-5": 2.00,
			},
		},
		{
			ByModel: map[string]float64{
				"claude-sonnet-4-5": 1.00,
				"claude-haiku-3-5":  0.50,
			},
		},
	}

	breakdown := ModelBreakdown(stats)
	if len(breakdown) != 3 {
		t.Fatalf("expected 3 models, got %d", len(breakdown))
	}

	// Should be sorted by cost descending
	if breakdown[0].Model != "claude-opus-4-6" || math.Abs(breakdown[0].Cost-10.00) > 0.001 {
		t.Errorf("top model = %s ($%.2f), want claude-opus-4-6 ($10.00)", breakdown[0].Model, breakdown[0].Cost)
	}
}

func TestGenerateSummary(t *testing.T) {
	records := []SessionRecord{
		{
			SessionID:         "s1",
			Project:           "codes",
			Model:             "claude-sonnet-4-5",
			StartTime:         time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC),
			CostUSD:           3.00,
			InputTokens:       100_000,
			OutputTokens:      50_000,
			CacheCreateTokens: 10_000,
			CacheReadTokens:   20_000,
		},
		{
			SessionID:         "s2",
			Project:           "conduit",
			Model:             "claude-opus-4-6",
			StartTime:         time.Date(2026, 2, 15, 14, 0, 0, 0, time.UTC),
			CostUSD:           5.00,
			InputTokens:       200_000,
			OutputTokens:      100_000,
			CacheCreateTokens: 15_000,
			CacheReadTokens:   25_000,
		},
	}

	summary := GenerateSummary(records, time.Time{}, time.Time{})

	// Check totals
	if math.Abs(summary.TotalCost-8.00) > 0.001 {
		t.Errorf("TotalCost = %.2f, want 8.00", summary.TotalCost)
	}
	if summary.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d, want 2", summary.TotalSessions)
	}
	if summary.InputTokens != 300_000 {
		t.Errorf("InputTokens = %d, want 300000", summary.InputTokens)
	}
	if summary.OutputTokens != 150_000 {
		t.Errorf("OutputTokens = %d, want 150000", summary.OutputTokens)
	}
	if summary.CacheCreate != 25_000 {
		t.Errorf("CacheCreate = %d, want 25000", summary.CacheCreate)
	}
	if summary.CacheRead != 45_000 {
		t.Errorf("CacheRead = %d, want 45000", summary.CacheRead)
	}

	// Check top project (opus should be first due to higher cost)
	if len(summary.TopProjects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(summary.TopProjects))
	}
	if summary.TopProjects[0].Project != "conduit" || math.Abs(summary.TopProjects[0].Cost-5.00) > 0.001 {
		t.Errorf("top project = %s ($%.2f), want conduit ($5.00)", summary.TopProjects[0].Project, summary.TopProjects[0].Cost)
	}

	// Check daily breakdown
	if len(summary.DailyBreakdown) != 1 {
		t.Fatalf("expected 1 daily stat, got %d", len(summary.DailyBreakdown))
	}
	if summary.DailyBreakdown[0].Date != "2026-02-15" {
		t.Errorf("daily date = %s, want 2026-02-15", summary.DailyBreakdown[0].Date)
	}
}
