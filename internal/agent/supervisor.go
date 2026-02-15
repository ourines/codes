package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SupervisorConfig holds configuration for the daemon supervisor.
type SupervisorConfig struct {
	TeamName      string
	AgentName     string
	MaxRestarts   int           // maximum consecutive restarts before giving up (0 = unlimited)
	BackoffBase   time.Duration // base backoff duration (default: 1s)
	BackoffMax    time.Duration // maximum backoff duration (default: 60s)
	HealthCheck   time.Duration // health check interval (default: 30s)
	CrashWindow   time.Duration // time window to consider crashes as consecutive (default: 5m)
}

// Supervisor manages the lifecycle of an agent daemon, automatically restarting
// it on crashes with exponential backoff.
type Supervisor struct {
	cfg    SupervisorConfig
	logger *log.Logger
}

// NewSupervisor creates a new supervisor for an agent daemon.
func NewSupervisor(teamName, agentName string) *Supervisor {
	return &Supervisor{
		cfg: SupervisorConfig{
			TeamName:    teamName,
			AgentName:   agentName,
			MaxRestarts: 0, // unlimited by default
			BackoffBase: 1 * time.Second,
			BackoffMax:  60 * time.Second,
			HealthCheck: 30 * time.Second,
			CrashWindow: 5 * time.Minute,
		},
		logger: log.New(os.Stderr, fmt.Sprintf("[supervisor:%s] ", agentName), log.LstdFlags),
	}
}

// Run starts the supervisor loop, which will continuously restart the daemon
// until the context is cancelled or MaxRestarts is reached.
func (s *Supervisor) Run(ctx context.Context) error {
	s.logger.Printf("supervisor started for %s/%s", s.cfg.TeamName, s.cfg.AgentName)

	consecutiveRestarts := 0
	var lastCrashTime time.Time

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("supervisor shutting down")
			return ctx.Err()
		default:
		}

		// Clean up stale PID before starting
		if err := s.cleanupStalePID(); err != nil {
			s.logger.Printf("warning: failed to cleanup stale PID: %v", err)
		}

		// Start the daemon
		s.logger.Printf("starting daemon (restart count: %d)", consecutiveRestarts)
		err := s.startDaemon(ctx)

		// Daemon exited
		now := time.Now()

		// Check if this is a crash or graceful shutdown
		if err == context.Canceled {
			s.logger.Println("daemon stopped gracefully (context cancelled)")
			return nil
		}

		// Check if daemon was stopped via stop command
		state, _ := GetAgentState(s.cfg.TeamName, s.cfg.AgentName)
		if state != nil && state.Status == AgentStopped {
			s.logger.Println("daemon stopped gracefully (stop command)")
			return nil
		}

		// Record crash
		s.logger.Printf("daemon crashed: %v", err)
		s.recordCrash()

		// Reset consecutive counter if enough time has passed since last crash
		if !lastCrashTime.IsZero() && now.Sub(lastCrashTime) > s.cfg.CrashWindow {
			s.logger.Printf("crash window expired, resetting consecutive restart count")
			consecutiveRestarts = 0
		}

		lastCrashTime = now
		consecutiveRestarts++

		// Check if max restarts reached
		if s.cfg.MaxRestarts > 0 && consecutiveRestarts > s.cfg.MaxRestarts {
			s.logger.Printf("max restarts (%d) exceeded, giving up", s.cfg.MaxRestarts)
			return fmt.Errorf("max restarts exceeded after %d attempts", consecutiveRestarts)
		}

		// Calculate backoff with exponential increase
		backoff := s.calculateBackoff(consecutiveRestarts)
		s.logger.Printf("waiting %v before restart (attempt %d)", backoff, consecutiveRestarts+1)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			// Continue to next restart
		}
	}
}

// startDaemon spawns the agent daemon process and waits for it to exit.
func (s *Supervisor) startDaemon(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.CommandContext(ctx, exe, "agent", "run", s.cfg.TeamName, s.cfg.AgentName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Mark state as supervised before starting
	state, _ := GetAgentState(s.cfg.TeamName, s.cfg.AgentName)
	if state == nil {
		state = &AgentState{
			Name:       s.cfg.AgentName,
			Team:       s.cfg.TeamName,
			Supervised: true,
		}
	} else {
		state.Supervised = true
	}
	SaveAgentState(state)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	s.logger.Printf("daemon started (pid %d)", cmd.Process.Pid)

	// Wait for daemon to exit
	return cmd.Wait()
}

// recordCrash records a crash event in the agent state.
func (s *Supervisor) recordCrash() {
	state, err := GetAgentState(s.cfg.TeamName, s.cfg.AgentName)
	if err != nil || state == nil {
		s.logger.Printf("warning: cannot load state to record crash: %v", err)
		return
	}

	now := time.Now()
	state.LastCrash = &now
	state.RestartCount++
	state.Status = AgentStopped // temporarily stopped while restarting

	if err := SaveAgentState(state); err != nil {
		s.logger.Printf("warning: failed to save crash state: %v", err)
	}
}

// calculateBackoff computes the backoff duration using exponential backoff.
// Formula: min(BackoffBase * 2^(attempt-1), BackoffMax)
func (s *Supervisor) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return s.cfg.BackoffBase
	}

	// Exponential: 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
	backoff := s.cfg.BackoffBase
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff >= s.cfg.BackoffMax {
			return s.cfg.BackoffMax
		}
	}

	return backoff
}

// cleanupStalePID removes stale PID files and state when process is not running.
func (s *Supervisor) cleanupStalePID() error {
	state, err := GetAgentState(s.cfg.TeamName, s.cfg.AgentName)
	if err != nil || state == nil {
		return nil // no state to clean
	}

	if state.PID <= 0 {
		return nil
	}

	// Check if process is alive
	if isProcessAlive(state.PID) {
		return nil // process still running, no cleanup needed
	}

	s.logger.Printf("detected stale PID %d, cleaning up", state.PID)

	// Update state to reflect stopped status
	state.PID = 0
	state.Status = AgentStopped
	state.CurrentTask = 0

	return SaveAgentState(state)
}

// HealthCheck verifies that the daemon process is still running and updates
// the state if it has died unexpectedly.
func HealthCheck(teamName, agentName string) error {
	state, err := GetAgentState(teamName, agentName)
	if err != nil {
		return fmt.Errorf("cannot load agent state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("agent state not found")
	}

	if state.PID <= 0 {
		return fmt.Errorf("no PID recorded")
	}

	// Check if process is alive
	alive := isProcessAlive(state.PID)
	if !alive {
		// Process died unexpectedly (not under supervisor)
		if !state.Supervised {
			now := time.Now()
			state.LastCrash = &now
		}

		state.Status = AgentStopped
		state.CurrentTask = 0
		state.PID = 0

		if err := SaveAgentState(state); err != nil {
			return fmt.Errorf("failed to update state: %w", err)
		}

		return fmt.Errorf("process %d is not running", state.PID)
	}

	return nil
}

// GetAgentUptime returns the duration the agent has been running.
func GetAgentUptime(teamName, agentName string) (time.Duration, error) {
	state, err := GetAgentState(teamName, agentName)
	if err != nil {
		return 0, err
	}
	if state == nil {
		return 0, fmt.Errorf("agent not found")
	}

	if state.Status == AgentStopped {
		return 0, nil
	}

	return time.Since(state.StartedAt), nil
}

// HealthStatus holds structured health status for an agent.
type HealthStatus struct {
	Alive        bool          `json:"alive"`
	PID          int           `json:"pid"`
	Status       AgentStatus   `json:"status"`
	Uptime       time.Duration `json:"uptime"`
	RestartCount int           `json:"restartCount"`
	LastCrash    *time.Time    `json:"lastCrash,omitempty"`
	Supervised   bool          `json:"supervised"`
	Error        string        `json:"error,omitempty"`
}

// GetAgentHealthStatus returns the health status of an agent.
func GetAgentHealthStatus(teamName, agentName string) (*HealthStatus, error) {
	state, err := GetAgentState(teamName, agentName)
	if err != nil {
		return nil, fmt.Errorf("cannot load agent state: %w", err)
	}
	if state == nil {
		return &HealthStatus{
			Alive: false,
			Error: "agent not found",
		}, nil
	}

	status := &HealthStatus{
		PID:          state.PID,
		Status:       state.Status,
		RestartCount: state.RestartCount,
		LastCrash:    state.LastCrash,
		Supervised:   state.Supervised,
	}

	// Check if process is alive
	if state.PID > 0 {
		status.Alive = isProcessAlive(state.PID)
	}

	// Calculate uptime if running
	if status.Alive && state.Status != AgentStopped {
		status.Uptime = time.Since(state.StartedAt)
	}

	// Health check
	if err := HealthCheck(teamName, agentName); err != nil {
		status.Error = err.Error()
	}

	return status, nil
}

// StaleStateCleanup scans all agent states and cleans up stale PIDs.
// This should be called periodically (e.g., on startup or via cron).
func StaleStateCleanup() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot get home dir: %w", err)
	}

	agentDir := filepath.Join(home, ".codes", "agent")
	teams, err := os.ReadDir(agentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no agents directory
		}
		return fmt.Errorf("cannot read agent dir: %w", err)
	}

	cleanedCount := 0
	for _, team := range teams {
		if !team.IsDir() {
			continue
		}

		teamName := team.Name()
		agentsPath := filepath.Join(agentDir, teamName, "agents")

		agents, err := os.ReadDir(agentsPath)
		if err != nil {
			continue
		}

		for _, agentFile := range agents {
			if filepath.Ext(agentFile.Name()) != ".json" {
				continue
			}

			agentName := agentFile.Name()[:len(agentFile.Name())-5] // remove .json

			state, err := GetAgentState(teamName, agentName)
			if err != nil || state == nil {
				continue
			}

			if state.PID > 0 && !isProcessAlive(state.PID) {
				state.PID = 0
				state.Status = AgentStopped
				state.CurrentTask = 0
				SaveAgentState(state)
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		log.Printf("cleaned up %d stale agent state(s)", cleanedCount)
	}

	return nil
}
