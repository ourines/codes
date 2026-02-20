package scheduler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ScheduleType distinguishes one-shot vs recurring schedules.
type ScheduleType string

const (
	TypeOnce     ScheduleType = "once"
	TypePeriodic ScheduleType = "periodic"
)

// Schedule represents a single scheduled task.
type Schedule struct {
	ID        string       `json:"id"`
	Type      ScheduleType `json:"type"`
	Message   string       `json:"message"`    // sent to assistant when triggered
	SessionID string       `json:"session_id"` // which assistant session receives the trigger

	// TypeOnce: trigger at this absolute time.
	At *time.Time `json:"at,omitempty"`

	// TypePeriodic: standard cron expression (5-field: min hour dom mon dow).
	Cron string `json:"cron,omitempty"`

	// Runtime state.
	CreatedAt time.Time  `json:"created_at"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	Enabled   bool       `json:"enabled"`
}

// schedulesPath returns the path to the schedules file (~/.codes/assistant/schedules.json).
func schedulesPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	dir := filepath.Join(home, ".codes", "assistant")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	return filepath.Join(dir, "schedules.json"), nil
}

// LoadSchedules reads all schedules from disk.
// Returns an empty slice if the file does not exist yet.
func LoadSchedules() ([]*Schedule, error) {
	path, err := schedulesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []*Schedule{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read schedules: %w", err)
	}
	var schedules []*Schedule
	if err := json.Unmarshal(data, &schedules); err != nil {
		// Corrupted file — start fresh rather than crashing.
		return []*Schedule{}, nil
	}
	return schedules, nil
}

// SaveSchedules atomically writes the full schedule list to disk.
func SaveSchedules(schedules []*Schedule) error {
	path, err := schedulesPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(schedules, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal schedules: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// AddSchedule generates an ID for the schedule and appends it to disk.
func AddSchedule(s *Schedule) error {
	if s.ID == "" {
		s.ID = generateScheduleID()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	schedules, err := LoadSchedules()
	if err != nil {
		return err
	}
	schedules = append(schedules, s)
	return SaveSchedules(schedules)
}

// RemoveSchedule deletes the schedule with the given ID from disk.
// Returns nil if the ID was not found (idempotent).
func RemoveSchedule(id string) error {
	schedules, err := LoadSchedules()
	if err != nil {
		return err
	}
	filtered := schedules[:0]
	for _, s := range schedules {
		if s.ID != id {
			filtered = append(filtered, s)
		}
	}
	return SaveSchedules(filtered)
}

// ListSchedules is an alias for LoadSchedules provided for callers that
// want explicit list semantics.
func ListSchedules() ([]*Schedule, error) {
	return LoadSchedules()
}

// generateScheduleID creates a short random hex ID for a new schedule.
func generateScheduleID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
