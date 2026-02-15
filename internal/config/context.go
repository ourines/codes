package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// loadConfigFunc is the config loader function, overridable in tests.
var loadConfigFunc = LoadConfig

// ProjectLink defines a relationship between projects.
type ProjectLink struct {
	Name            string   `json:"name"`                        // linked project name
	Role            string   `json:"role,omitempty"`               // e.g. "API provider", "deployment target"
	AutoInjectPaths []string `json:"autoInjectPaths,omitempty"`    // file paths to inject as context
}

// LinkProject creates a link between two projects.
func LinkProject(projectName, linkedProject, role string) error {
	cfg, err := loadConfigFunc()
	if err != nil {
		return err
	}

	entry, exists := cfg.Projects[projectName]
	if !exists {
		return fmt.Errorf("project %q not found", projectName)
	}

	// Verify linked project exists
	if _, exists := cfg.Projects[linkedProject]; !exists {
		return fmt.Errorf("linked project %q not found", linkedProject)
	}

	// Check for duplicate link
	for _, link := range entry.Links {
		if link.Name == linkedProject {
			return fmt.Errorf("project %q is already linked to %q", projectName, linkedProject)
		}
	}

	entry.Links = append(entry.Links, ProjectLink{
		Name: linkedProject,
		Role: role,
	})
	cfg.Projects[projectName] = entry
	return SaveConfig(cfg)
}

// UnlinkProject removes a link between two projects.
func UnlinkProject(projectName, linkedProject string) error {
	cfg, err := loadConfigFunc()
	if err != nil {
		return err
	}

	entry, exists := cfg.Projects[projectName]
	if !exists {
		return fmt.Errorf("project %q not found", projectName)
	}

	found := false
	filtered := make([]ProjectLink, 0, len(entry.Links))
	for _, link := range entry.Links {
		if link.Name == linkedProject {
			found = true
			continue
		}
		filtered = append(filtered, link)
	}

	if !found {
		return fmt.Errorf("project %q is not linked to %q", projectName, linkedProject)
	}

	entry.Links = filtered
	cfg.Projects[projectName] = entry
	return SaveConfig(cfg)
}

// GetLinkedProjectsSummary generates a context summary of linked projects.
func GetLinkedProjectsSummary(projectName string) (string, error) {
	cfg, err := loadConfigFunc()
	if err != nil {
		return "", err
	}

	entry, exists := cfg.Projects[projectName]
	if !exists {
		return "", fmt.Errorf("project %q not found", projectName)
	}

	if len(entry.Links) == 0 {
		return "", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Linked Projects for %s\n\n", projectName))

	for _, link := range entry.Links {
		linkedEntry, exists := cfg.Projects[link.Name]
		if !exists {
			continue
		}

		b.WriteString(fmt.Sprintf("## %s", link.Name))
		if link.Role != "" {
			b.WriteString(fmt.Sprintf(" (%s)", link.Role))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("Path: %s\n", linkedEntry.Path))

		// Inject file contents if configured
		for _, relPath := range link.AutoInjectPaths {
			fullPath := filepath.Join(linkedEntry.Path, relPath)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			b.WriteString(fmt.Sprintf("\n### %s\n```\n%s\n```\n", relPath, strings.TrimSpace(string(content))))
		}

		// Check for CLAUDE.md in linked project
		claudePath := filepath.Join(linkedEntry.Path, "CLAUDE.md")
		if _, err := os.Stat(claudePath); err == nil {
			content, err := os.ReadFile(claudePath)
			if err == nil {
				summary := strings.TrimSpace(string(content))
				if len(summary) > 500 {
					summary = summary[:500] + "..."
				}
				b.WriteString(fmt.Sprintf("\n### CLAUDE.md (summary)\n%s\n", summary))
			}
		}
		b.WriteString("\n")
	}

	return b.String(), nil
}

// LinkedContextArgs returns extra Claude CLI args to inject linked project context.
// Returns nil if no links exist for the project.
func LinkedContextArgs(projectName string) []string {
	summary, err := GetLinkedProjectsSummary(projectName)
	if err != nil || summary == "" {
		return nil
	}
	return []string{"--append-system-prompt", summary}
}
