package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
)

// isSensitive checks if an environment variable key contains sensitive information.
func isSensitive(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "TOKEN") ||
		strings.Contains(upper, "KEY") ||
		strings.Contains(upper, "SECRET") ||
		strings.Contains(upper, "PASSWORD")
}

// redactConfig returns a deep copy of the config with sensitive env vars replaced by <REDACTED>.
func redactConfig(cfg *config.Config) *config.Config {
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg
	}
	var cp config.Config
	if err := json.Unmarshal(data, &cp); err != nil {
		return cfg
	}

	for i := range cp.Profiles {
		for k := range cp.Profiles[i].Env {
			if isSensitive(k) {
				cp.Profiles[i].Env[k] = "<REDACTED>"
			}
		}
	}

	return &cp
}

// mergeConfig merges imported config into existing config.
// Redacted values (<REDACTED>) are skipped, preserving local values.
func mergeConfig(existing, imported *config.Config) {
	// Merge profiles
	for _, imp := range imported.Profiles {
		found := false
		for i, ex := range existing.Profiles {
			if ex.Name == imp.Name {
				found = true
				if existing.Profiles[i].Env == nil {
					existing.Profiles[i].Env = make(map[string]string)
				}
				for k, v := range imp.Env {
					if v == "<REDACTED>" {
						continue
					}
					existing.Profiles[i].Env[k] = v
				}
				if imp.SkipPermissions != nil {
					existing.Profiles[i].SkipPermissions = imp.SkipPermissions
				}
				break
			}
		}
		if !found {
			cleanProfile := config.APIConfig{
				Name:            imp.Name,
				Env:             make(map[string]string),
				SkipPermissions: imp.SkipPermissions,
				Status:          imp.Status,
			}
			for k, v := range imp.Env {
				if v != "<REDACTED>" {
					cleanProfile.Env[k] = v
				}
			}
			existing.Profiles = append(existing.Profiles, cleanProfile)
		}
	}

	// Merge projects
	if existing.Projects == nil {
		existing.Projects = make(map[string]config.ProjectEntry)
	}
	for name, entry := range imported.Projects {
		existing.Projects[name] = entry
	}

	// Merge remotes
	for _, imp := range imported.Remotes {
		found := false
		for i, ex := range existing.Remotes {
			if ex.Name == imp.Name {
				found = true
				existing.Remotes[i] = imp
				break
			}
		}
		if !found {
			existing.Remotes = append(existing.Remotes, imp)
		}
	}

	// Merge webhooks
	for _, imp := range imported.Webhooks {
		found := false
		for i, ex := range existing.Webhooks {
			if ex.Name == imp.Name {
				found = true
				existing.Webhooks[i] = imp
				break
			}
		}
		if !found {
			existing.Webhooks = append(existing.Webhooks, imp)
		}
	}

	// Merge scalar settings
	if imported.Default != "" {
		existing.Default = imported.Default
	}
	if imported.DefaultBehavior != "" {
		existing.DefaultBehavior = imported.DefaultBehavior
	}
	if imported.Terminal != "" {
		existing.Terminal = imported.Terminal
	}
	if imported.ProjectsDir != "" {
		existing.ProjectsDir = imported.ProjectsDir
	}
	if imported.AutoUpdate != "" {
		existing.AutoUpdate = imported.AutoUpdate
	}
	if imported.Editor != "" {
		existing.Editor = imported.Editor
	}
}

// RunConfigExport exports the current configuration to stdout or a file.
func RunConfigExport(filename string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Failed to load config", err)
		return
	}

	redacted := redactConfig(cfg)

	data, err := json.MarshalIndent(redacted, "", "    ")
	if err != nil {
		ui.ShowError("Failed to marshal config", err)
		return
	}

	if filename != "" {
		if err := os.WriteFile(filename, data, 0600); err != nil {
			ui.ShowError("Failed to write file", err)
			return
		}
		ui.ShowSuccess("Config exported to %s", filename)
	} else {
		fmt.Println(string(data))
	}
}

// RunConfigImport imports configuration from a file, merging with existing config.
func RunConfigImport(filename string) {
	data, err := os.ReadFile(filename)
	if err != nil {
		ui.ShowError("Failed to read import file", err)
		return
	}

	var imported config.Config
	if err := json.Unmarshal(data, &imported); err != nil {
		ui.ShowError("Failed to parse import file", err)
		return
	}

	existing, err := config.LoadConfig()
	if err != nil {
		ui.ShowError("Failed to load existing config", err)
		return
	}

	mergeConfig(existing, &imported)

	if err := config.SaveConfig(existing); err != nil {
		ui.ShowError("Failed to save config", err)
		return
	}

	ui.ShowSuccess("Config imported from %s (merged with existing)", filename)
}
