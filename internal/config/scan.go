package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DiscoveredProject represents a project found by scanning ~/.claude/projects/
type DiscoveredProject struct {
	Path         string    // Full filesystem path, e.g. /Users/ourines/Projects/codes
	Name         string    // Auto-generated alias (last segment of path, e.g. "codes")
	HasClaude    bool      // Whether CLAUDE.md exists in the project root
	LastActive   time.Time // Most recent session file modification time
	SessionCount int       // Number of session files
}

// ScanClaudeProjects scans ~/.claude/projects/ and returns discovered projects.
// It decodes the encoded directory names back to real filesystem paths,
// validates they exist, and gathers metadata about each project.
func ScanClaudeProjects() ([]DiscoveredProject, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	claudeProjectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No Claude projects directory yet
		}
		return nil, fmt.Errorf("cannot read %s: %w", claudeProjectsDir, err)
	}

	var discovered []DiscoveredProject

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		encoded := entry.Name()
		decoded := decodeClaudeProjectPath(encoded)
		if decoded == "" {
			continue // Could not decode or path doesn't exist
		}

		// Gather metadata
		proj := DiscoveredProject{
			Path:      decoded,
			Name:      filepath.Base(decoded),
			HasClaude: hasClaudeMD(decoded),
		}

		// Count session files and find the most recent one
		sessionDir := filepath.Join(claudeProjectsDir, encoded)
		sessionEntries, err := os.ReadDir(sessionDir)
		if err == nil {
			for _, se := range sessionEntries {
				if !se.IsDir() && strings.HasSuffix(se.Name(), ".jsonl") {
					proj.SessionCount++
					if info, err := se.Info(); err == nil {
						if info.ModTime().After(proj.LastActive) {
							proj.LastActive = info.ModTime()
						}
					}
				}
			}
		}

		discovered = append(discovered, proj)
	}

	// Sort by most recently active first
	sort.Slice(discovered, func(i, j int) bool {
		return discovered[i].LastActive.After(discovered[j].LastActive)
	})

	return discovered, nil
}

// decodeClaudeProjectPath converts Claude's encoded directory name back to a real path.
// Claude encodes paths by replacing "/" with "-", e.g.:
//
//	"-Users-ourines-Projects-codes" → "/Users/ourines/Projects/codes"
//
// The challenge is that paths may contain literal hyphens (e.g. "crs-local"),
// so we use a greedy directory-matching algorithm that validates against the filesystem.
func decodeClaudeProjectPath(encoded string) string {
	if encoded == "" || encoded[0] != '-' {
		return ""
	}

	// Remove leading dash and split by dash
	parts := strings.Split(encoded[1:], "-")
	if len(parts) == 0 {
		return ""
	}

	// Use greedy directory-segment matching:
	// Start from root "/", try to match the longest possible segment
	// that corresponds to a real directory or file.
	return greedyPathResolve("/", parts, 0)
}

// greedyPathResolve attempts to reconstruct a real path from dash-separated segments.
// It tries matching progressively longer segments (joining with "-") against the filesystem.
// The algorithm prefers shorter segments (more path components) to greedily build the path.
func greedyPathResolve(base string, parts []string, idx int) string {
	if idx >= len(parts) {
		// All parts consumed — check if the path exists
		if pathExists(base) {
			return base
		}
		return ""
	}

	// Try consuming 1, 2, 3... parts as a single path segment
	// Prefer fewer parts first (i.e., treat dash as path separator when possible)
	for end := idx + 1; end <= len(parts); end++ {
		segment := strings.Join(parts[idx:end], "-")
		candidate := filepath.Join(base, segment)

		if end == len(parts) {
			// Last segment — the full path must exist
			if pathExists(candidate) {
				return candidate
			}
		} else {
			// Middle segment — must be a directory
			if isDir(candidate) {
				result := greedyPathResolve(candidate, parts, end)
				if result != "" {
					return result
				}
			}
		}
	}

	return "" // No valid path found
}

// ImportDiscoveredProjects adds new projects to the config, skipping existing ones.
// Returns the number of projects added and skipped.
func ImportDiscoveredProjects(projects []DiscoveredProject) (added int, skipped int, err error) {
	cfg, err := LoadConfig()
	if err != nil {
		return 0, 0, err
	}

	if cfg.Projects == nil {
		cfg.Projects = make(map[string]ProjectEntry)
	}

	// Build a set of existing paths for dedup
	existingPaths := make(map[string]bool)
	for _, entry := range cfg.Projects {
		existingPaths[entry.Path] = true
	}

	for _, proj := range projects {
		// Skip if path already registered (regardless of alias name)
		if existingPaths[proj.Path] {
			skipped++
			continue
		}

		// Generate unique alias name
		name := uniqueAlias(proj.Name, cfg.Projects)

		cfg.Projects[name] = ProjectEntry{Path: proj.Path}
		existingPaths[proj.Path] = true
		added++
	}

	if added > 0 {
		if err := SaveConfig(cfg); err != nil {
			return 0, 0, fmt.Errorf("failed to save config: %w", err)
		}
	}

	return added, skipped, nil
}

// uniqueAlias generates a unique alias name, appending a numeric suffix if needed.
// e.g., if "codes" exists, returns "codes-2", then "codes-3", etc.
func uniqueAlias(base string, existing map[string]ProjectEntry) string {
	if _, exists := existing[base]; !exists {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if _, exists := existing[candidate]; !exists {
			return candidate
		}
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
