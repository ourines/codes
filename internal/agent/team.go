package agent

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

// CreateTeam creates a new team workspace with the given configuration.
func CreateTeam(name, description, workDir string) (*TeamConfig, error) {
	dir := teamDir(name)
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("team %q already exists", name)
	}

	// Create directory structure
	for _, d := range []string{
		dir,
		tasksDir(name),
		messagesDir(name),
		agentsDir(name),
	} {
		if err := ensureDir(d); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}

	cfg := &TeamConfig{
		Name:        name,
		Description: description,
		WorkDir:     workDir,
		Members:     []TeamMember{},
		CreatedAt:   time.Now(),
	}

	if err := writeJSON(teamConfigPath(name), cfg); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("write config: %w", err)
	}

	return cfg, nil
}

// GetTeam loads a team's configuration.
func GetTeam(name string) (*TeamConfig, error) {
	var cfg TeamConfig
	if err := readJSON(teamConfigPath(name), &cfg); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("team %q not found", name)
		}
		return nil, err
	}
	return &cfg, nil
}

// ListTeams returns the names of all teams.
func ListTeams() ([]string, error) {
	base := teamsBaseDirFunc()
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			// Verify it has a config.json
			if _, err := os.Stat(teamConfigPath(e.Name())); err == nil {
				names = append(names, e.Name())
			}
		}
	}
	return names, nil
}

// DeleteTeam removes a team and all its data.
func DeleteTeam(name string) error {
	dir := teamDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("team %q not found", name)
	}
	return os.RemoveAll(dir)
}

// AddMember registers a new agent in the team.
func AddMember(teamName string, member TeamMember) error {
	cfg, err := GetTeam(teamName)
	if err != nil {
		return err
	}

	for _, m := range cfg.Members {
		if m.Name == member.Name {
			return fmt.Errorf("member %q already exists in team %q", member.Name, teamName)
		}
	}

	cfg.Members = append(cfg.Members, member)
	return writeJSON(teamConfigPath(teamName), cfg)
}

// RemoveMember removes an agent from the team.
func RemoveMember(teamName, memberName string) error {
	cfg, err := GetTeam(teamName)
	if err != nil {
		return err
	}

	found := false
	members := make([]TeamMember, 0, len(cfg.Members))
	for _, m := range cfg.Members {
		if m.Name == memberName {
			found = true
			continue
		}
		members = append(members, m)
	}

	if !found {
		return fmt.Errorf("member %q not found in team %q", memberName, teamName)
	}

	cfg.Members = members

	// Remove agent state file if exists
	os.Remove(agentStatePath(teamName, memberName))

	return writeJSON(teamConfigPath(teamName), cfg)
}

// GetAgentState loads an agent's runtime state.
func GetAgentState(teamName, agentName string) (*AgentState, error) {
	var state AgentState
	if err := readJSON(agentStatePath(teamName, agentName), &state); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

// SaveAgentState writes an agent's runtime state to disk.
func SaveAgentState(state *AgentState) error {
	state.UpdatedAt = time.Now()
	return writeJSON(agentStatePath(state.Team, state.Name), state)
}

// IsAgentAlive checks if an agent's daemon process is still running.
// If the process is dead but the recorded status is not AgentStopped,
// the state is updated to AgentStopped automatically.
func IsAgentAlive(teamName, agentName string) bool {
	state, err := GetAgentState(teamName, agentName)
	if err != nil || state == nil {
		return false
	}

	if state.PID <= 0 {
		return false
	}

	alive := isProcessAlive(state.PID)
	if !alive && state.Status != AgentStopped {
		state.Status = AgentStopped
		state.CurrentTask = 0
		SaveAgentState(state)
	}
	return alive
}

// AgentStartResult holds the result of starting a single agent.
type AgentStartResult struct {
	Name    string `json:"name"`
	Started bool   `json:"started"`
	PID     int    `json:"pid,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StartAgent spawns an agent daemon as an independent subprocess.
// Returns the PID of the spawned process.
func StartAgent(teamName, agentName string) (int, error) {
	// Verify the agent exists
	if _, err := NewDaemon(teamName, agentName); err != nil {
		return 0, err
	}

	// Check if already running
	if IsAgentAlive(teamName, agentName) {
		state, _ := GetAgentState(teamName, agentName)
		pid := 0
		if state != nil {
			pid = state.PID
		}
		return 0, fmt.Errorf("agent %q is already running (pid %d)", agentName, pid)
	}

	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(exe, "agent", "run", teamName, agentName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start agent: %w", err)
	}

	pid := cmd.Process.Pid
	cmd.Process.Release() // detach
	return pid, nil
}

// StartAllAgents spawns all agents in a team, skipping those already running.
func StartAllAgents(teamName string) ([]AgentStartResult, error) {
	cfg, err := GetTeam(teamName)
	if err != nil {
		return nil, err
	}

	var results []AgentStartResult
	for _, m := range cfg.Members {
		r := AgentStartResult{Name: m.Name}

		if IsAgentAlive(teamName, m.Name) {
			state, _ := GetAgentState(teamName, m.Name)
			if state != nil {
				r.PID = state.PID
			}
			r.Error = "already running"
			results = append(results, r)
			continue
		}

		pid, err := StartAgent(teamName, m.Name)
		if err != nil {
			// StartAgent checks alive again; handle "already running" gracefully
			r.Error = err.Error()
			results = append(results, r)
			continue
		}

		r.Started = true
		r.PID = pid
		results = append(results, r)
	}

	return results, nil
}
