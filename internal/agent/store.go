package agent

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// teamsBaseDirFunc returns the base directory for all teams (~/.codes/teams/).
// It's a variable so tests can override it.
var teamsBaseDirFunc = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codes", "teams")
}

// teamDir returns the directory for a specific team.
func teamDir(teamName string) string {
	return filepath.Join(teamsBaseDirFunc(), teamName)
}

// teamConfigPath returns the path to the team config file.
func teamConfigPath(teamName string) string {
	return filepath.Join(teamDir(teamName), "config.json")
}

// tasksDir returns the tasks directory for a team.
func tasksDir(teamName string) string {
	return filepath.Join(teamDir(teamName), "tasks")
}

// taskPath returns the path to a specific task file.
func taskPath(teamName string, taskID int) string {
	return filepath.Join(tasksDir(teamName), fmt.Sprintf("%d.json", taskID))
}

// taskLockPath returns the path to a task's lock file.
func taskLockPath(teamName string, taskID int) string {
	return filepath.Join(tasksDir(teamName), fmt.Sprintf("%d.json.lock", taskID))
}

// messagesDir returns the messages directory for a team.
func messagesDir(teamName string) string {
	return filepath.Join(teamDir(teamName), "messages")
}

// agentsDir returns the agents directory for a team.
func agentsDir(teamName string) string {
	return filepath.Join(teamDir(teamName), "agents")
}

// agentStatePath returns the path to an agent's state file.
func agentStatePath(teamName, agentName string) string {
	return filepath.Join(agentsDir(teamName), agentName+".json")
}

// ensureDir creates a directory (and parents) if it doesn't exist.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// writeJSON atomically writes a value as JSON to a file.
// It writes to a .tmp file first, then renames for crash safety.
func writeJSON(path string, v any) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
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

// readJSON reads and unmarshals a JSON file into v.
func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// nextTaskID scans the tasks directory and returns the next available ID.
func nextTaskID(teamName string) (int, error) {
	dir := tasksDir(teamName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}

	maxID := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var id int
		if _, err := fmt.Sscanf(e.Name(), "%d.json", &id); err == nil {
			if id > maxID {
				maxID = id
			}
		}
	}
	return maxID + 1, nil
}

// generateID creates a random UUID v4 string for session IDs.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	// Set version 4 (random) and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]))
}
