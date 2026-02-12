package tui

import (
	"fmt"
	"strings"

	"codes/internal/config"
	"codes/internal/remote"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

// remoteItem implements list.Item for remote host entries.
type remoteItem struct {
	host config.RemoteHost
}

func (i remoteItem) Title() string       { return i.host.Name }
func (i remoteItem) Description() string {
	desc := i.host.UserAtHost()
	if i.host.Port != 0 && i.host.Port != 22 {
		desc += fmt.Sprintf(":%d", i.host.Port)
	}
	return desc
}
func (i remoteItem) FilterValue() string { return i.host.Name + " " + i.host.UserAtHost() }

// loadRemotes returns a list of remote hosts as list.Item.
func loadRemotes() []list.Item {
	remotes, err := config.ListRemotes()
	if err != nil {
		return nil
	}

	items := make([]list.Item, len(remotes))
	for i, r := range remotes {
		items[i] = remoteItem{host: r}
	}
	return items
}

// renderRemoteDetail renders the right-side detail panel for a remote host.
func renderRemoteDetail(host config.RemoteHost, width, height int, status *remote.RemoteStatus) string {
	var b strings.Builder

	// Name
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		detailLabelStyle.Render("Name:"),
		detailValueStyle.Render(host.Name)))

	// Host
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		detailLabelStyle.Render("Host:"),
		detailValueStyle.Render(host.UserAtHost())))

	// Port
	port := "22"
	if host.Port != 0 {
		port = fmt.Sprintf("%d", host.Port)
	}
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		detailLabelStyle.Render("Port:"),
		detailValueStyle.Render(port)))

	// Identity
	if host.Identity != "" {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			detailLabelStyle.Render("Key:"),
			detailValueStyle.Render(host.Identity)))
	}

	b.WriteString("\n")

	// Status info (if available)
	if status != nil {
		b.WriteString(fmt.Sprintf("  %s\n", detailLabelStyle.Render("Status:")))

		// Connection
		b.WriteString(fmt.Sprintf("    %s %s\n",
			statusOkStyle.Render("●"),
			detailValueStyle.Render("Connected")))

		// OS/Arch
		b.WriteString(fmt.Sprintf("    %s %s/%s\n",
			detailLabelStyle.Render("Platform:"),
			detailValueStyle.Render(status.OS),
			detailValueStyle.Render(status.Arch)))

		// codes
		if status.CodesInstalled {
			b.WriteString(fmt.Sprintf("    %s %s\n",
				statusOkStyle.Render("codes:"),
				detailValueStyle.Render(status.CodesVersion)))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n",
				statusWarnStyle.Render("codes: not installed")))
		}

		// claude
		if status.ClaudeInstalled {
			b.WriteString(fmt.Sprintf("    %s\n",
				statusOkStyle.Render("claude: installed")))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n",
				statusWarnStyle.Render("claude: not installed")))
		}
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n",
			detailLabelStyle.Render("Status:"),
			lipgloss.NewStyle().Foreground(mutedColor).Render("Press t to test")))
	}

	// Keybinding hints
	b.WriteString("\n")
	b.WriteString(formHintStyle.Render("  t: test  s: sync  S: setup  a: add  d: delete"))

	return detailBorderStyle.
		Width(width - 4).
		Height(height - 4).
		Render(b.String())
}
