package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors — low-saturation palette, easy on the eyes
	primaryColor   = lipgloss.Color("#7B9DB7") // dusty steel blue
	secondaryColor = lipgloss.Color("#8AAF9D") // dusty sage
	mutedColor     = lipgloss.Color("#6B7280") // gray
	dangerColor    = lipgloss.Color("#C08A83") // dusty rose
	warnColor      = lipgloss.Color("#C4AD88") // warm sand

	// App frame
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E8E4E0")).
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
				Foreground(lipgloss.Color("#C8CCD0"))

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

// newStyledDelegate returns a list delegate with colors matching our palette.
func newStyledDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.SetSpacing(0)
	d.Styles.SelectedTitle = d.Styles.SelectedTitle.
		Foreground(primaryColor).
		BorderLeftForeground(primaryColor)
	d.Styles.SelectedDesc = d.Styles.SelectedDesc.
		Foreground(mutedColor).
		BorderLeftForeground(primaryColor)
	return d
}
