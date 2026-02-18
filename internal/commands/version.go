package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"codes/internal/config"
	"codes/internal/ui"
	"codes/internal/update"
)

// Version information, set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func RunVersion() {
	fmt.Printf("codes version %s (commit %s, built %s)\n", Version, Commit, Date)
}

func RunClaudeUpdate() {
	ui.ShowHeader("Claude Version Manager")
	ui.ShowLoading("Fetching available versions...")

	cmd := exec.Command("npm", "view", "@anthropic-ai/claude-code", "versions", "--json")
	npmOutput, err := cmd.Output()
	if err != nil {
		ui.ShowError("Failed to fetch Claude versions", nil)
		return
	}

	var versions []string
	if err := json.Unmarshal(npmOutput, &versions); err != nil {
		ui.ShowError("Failed to parse versions", nil)
		return
	}

	fmt.Println()
	ui.ShowInfo("Found %d available versions", len(versions))
	fmt.Println()

	ui.ShowInfo("Latest versions:")
	displayCount := 20
	if len(versions) < displayCount {
		displayCount = len(versions)
	}

	startIndex := len(versions) - displayCount
	for i := 0; i < displayCount; i++ {
		versionIndex := startIndex + i
		ui.ShowVersionItem(i+1, versions[versionIndex])
	}

	if len(versions) > displayCount {
		fmt.Println()
		ui.ShowInfo("(Showing %d most recent versions out of %d total)", displayCount, len(versions))
	}

	fmt.Println()
	fmt.Printf("Select version (1-%d, version number, or 'latest'): ", displayCount)
	reader := bufio.NewReader(os.Stdin)
	selection, _ := reader.ReadString('\n')
	selection = strings.TrimSpace(selection)

	if selection == "" {
		ui.ShowWarning("No selection made. Installing latest...")
		installClaude("latest")
		return
	}

	if selection == "latest" {
		ui.ShowLoading("Installing Claude latest...")
		installClaude("latest")
		return
	}

	if selectedIdx, err := strconv.Atoi(selection); err == nil && selectedIdx >= 1 && selectedIdx <= displayCount {
		versionIndex := startIndex + selectedIdx - 1
		selectedVersion := versions[versionIndex]
		ui.ShowLoading("Installing Claude %s...", selectedVersion)
		installClaude(selectedVersion)
		return
	}

	ui.ShowLoading("Installing Claude %s...", selection)
	installClaude(selection)
}

func checkForUpdates() {
	// Apply any previously staged update (synchronous)
	if err := update.ApplyStaged(); err != nil {
		ui.ShowWarning("Failed to apply staged update: %v", err)
	}

	// Async version check
	mode := config.GetAutoUpdate()
	go update.AutoCheck(Version, mode)
}

// RunSelfUpdate performs a manual codes self-update.
func RunSelfUpdate() {
	ui.ShowHeader("codes Self-Update")
	if err := update.RunSelfUpdate(Version); err != nil {
		ui.ShowError(err.Error(), nil)
		os.Exit(1)
	}
}
