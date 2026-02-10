package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // purple
	secondaryColor = lipgloss.Color("#10B981") // green
	mutedColor     = lipgloss.Color("#6B7280") // gray
	dangerColor    = lipgloss.Color("#EF4444") // red
	warnColor      = lipgloss.Color("#F59E0B") // yellow

	// App frame
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Padding(0, 1)

	// Active tab
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Underline(true)

	// Inactive tab
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	// Detail panel styles
	detailBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2)

	detailLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E7EB"))

	// Status indicators
	statusOkStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	statusWarnStyle = lipgloss.NewStyle().
			Foreground(warnColor)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(dangerColor)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(1, 0, 0, 0)

	// Form styles
	formLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	formInputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	formHintStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)
)
