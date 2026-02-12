package tui

import (
	"fmt"
	"strings"

	"codes/internal/config"

	"github.com/charmbracelet/bubbles/list"
)

// profileItem implements list.Item for API profiles.
type profileItem struct {
	cfg       config.APIConfig
	isDefault bool
}

func (i profileItem) Title() string {
	name := i.cfg.Name
	if i.isDefault {
		name += " ★"
	}
	return name
}

func (i profileItem) Description() string {
	var parts []string
	status := i.cfg.Status
	if status == "" {
		status = "unknown"
	}
	switch status {
	case "active":
		parts = append(parts, "● active")
	case "inactive":
		parts = append(parts, "○ inactive")
	default:
		parts = append(parts, "? "+status)
	}
	parts = append(parts, fmt.Sprintf("%d env", len(i.cfg.Env)))
	if i.cfg.SkipPermissions != nil && *i.cfg.SkipPermissions {
		parts = append(parts, "skip-perms")
	}
	return strings.Join(parts, "  ")
}

func (i profileItem) FilterValue() string {
	return i.cfg.Name + " " + i.cfg.Status
}

// loadProfiles loads all API profiles and returns them as list items
// along with the default profile name.
func loadProfiles() ([]list.Item, string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, ""
	}

	items := make([]list.Item, len(cfg.Profiles))
	for i, apiCfg := range cfg.Profiles {
		items[i] = profileItem{
			cfg:       apiCfg,
			isDefault: apiCfg.Name == cfg.Default,
		}
	}
	return items, cfg.Default
}

// renderProfileDetail renders the right-side detail panel for a profile item.
func renderProfileDetail(item profileItem, width, height int) string {
	var b strings.Builder

	// Name
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		detailLabelStyle.Render("Name:"),
		detailValueStyle.Render(item.cfg.Name)))

	// Status with color indicator
	status := item.cfg.Status
	if status == "" {
		status = "unknown"
	}
	var styledStatus string
	switch status {
	case "active":
		styledStatus = statusOkStyle.Render(status + " ✓")
	case "inactive":
		styledStatus = statusErrorStyle.Render(status + " ✗")
	default:
		styledStatus = statusWarnStyle.Render(status + " ?")
	}
	b.WriteString(fmt.Sprintf("  %s %s\n",
		detailLabelStyle.Render("Status:"),
		styledStatus))

	// Default indicator
	defaultVal := "no"
	if item.isDefault {
		defaultVal = "★ yes"
	}
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		detailLabelStyle.Render("Default:"),
		detailValueStyle.Render(defaultVal)))

	// Environment variable count
	envCount := len(item.cfg.Env)
	b.WriteString(fmt.Sprintf("  %s %s\n",
		detailLabelStyle.Render("Env vars:"),
		detailValueStyle.Render(fmt.Sprintf("%d", envCount))))

	// Skip permissions
	skipVal := "no"
	if item.cfg.SkipPermissions != nil && *item.cfg.SkipPermissions {
		skipVal = "yes"
	}
	b.WriteString(fmt.Sprintf("  %s     %s\n",
		detailLabelStyle.Render("Skip:"),
		detailValueStyle.Render(skipVal)))

	return detailBorderStyle.
		Width(width - 4).
		Height(height - 4).
		Render(b.String())
}
