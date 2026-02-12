package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"codes/internal/config"
)

type settingItem struct {
	label   string   // display label
	key     string   // internal key
	value   string   // current value
	options []string // available options (nil = read-only)
}

type settingsModel struct {
	items  []settingItem
	cursor int
}

// settingChangedMsg is sent after a setting value is changed.
type settingChangedMsg struct{}

func newSettingsModel(cfg *config.Config) settingsModel {
	terminal := "terminal"
	behavior := "current"
	skip := "off"
	projectsDir := config.GetProjectsDir()
	configFile := config.ConfigPath

	if cfg != nil {
		if cfg.Terminal != "" {
			terminal = cfg.Terminal
		}
		if cfg.DefaultBehavior != "" {
			behavior = cfg.DefaultBehavior
		}
		if cfg.SkipPermissions {
			skip = "on"
		}
		if cfg.ProjectsDir != "" {
			projectsDir = cfg.ProjectsDir
		}
	}

	return settingsModel{
		items: []settingItem{
			{
				label:   "Terminal",
				key:     "terminal",
				value:   terminal,
				options: config.TerminalOptions(),
			},
			{
				label:   "Projects Dir",
				key:     "projects_dir",
				value:   projectsDir,
				options: config.ProjectsDirOptions(),
			},
			{
				label:   "Default Behavior",
				key:     "defaultBehavior",
				value:   behavior,
				options: []string{"current", "last", "home"},
			},
			{
				label:   "Skip Permissions",
				key:     "skipPermissions",
				value:   skip,
				options: []string{"off", "on"},
			},
			{
				label:   "Config File",
				key:     "configFile",
				value:   configFile,
				options: nil, // read-only
			},
		},
	}
}

func (s settingsModel) View(width, height int) string {
	var b strings.Builder

	title := detailLabelStyle.Render("Settings")
	b.WriteString("  " + title + "\n\n")

	for i, item := range s.items {
		cursor := "  "
		labelStyle := formLabelStyle
		if i == s.cursor {
			cursor = "▸ "
			labelStyle = detailLabelStyle
		}

		label := labelStyle.Render(item.label)

		var valueStr string
		if item.options != nil {
			// Show current value with cycle hint
			valueStr = detailValueStyle.Render(item.value)
			if i == s.cursor {
				valueStr = statusOkStyle.Render(item.value)
			}
		} else {
			// Read-only
			valueStr = lipgloss.NewStyle().Foreground(mutedColor).Render(item.value)
		}

		b.WriteString(fmt.Sprintf("%s%-20s  %s\n", cursor, label, valueStr))

		// Show description for selected item
		if i == s.cursor {
			desc := settingDescription(item.key, item.value)
			if desc != "" {
				b.WriteString(fmt.Sprintf("  %-20s  %s\n", "", formHintStyle.Render(desc)))
			}
		}
		b.WriteString("\n")
	}

	return detailBorderStyle.
		Width(width - 4).
		Height(height - 4).
		Render(b.String())
}

func settingDescription(key, value string) string {
	switch key {
	case "terminal":
		switch value {
		case "terminal":
			return "macOS Terminal.app"
		case "iterm":
			return "iTerm2"
		case "warp":
			return "Warp terminal"
		default:
			return "Custom: " + value
		}
	case "projects_dir":
		return "Default directory for git clone"
	case "defaultBehavior":
		switch value {
		case "current":
			return "Start in current working directory"
		case "last":
			return "Start in last used directory"
		case "home":
			return "Start in home directory"
		}
	case "skipPermissions":
		if value == "on" {
			return "Claude runs with --dangerously-skip-permissions"
		}
		return "Claude runs with normal permission checks"
	case "configFile":
		return "Read-only"
	}
	return ""
}
