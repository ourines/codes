package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "ourines"
	repoName  = "codes"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// stateFilePath returns ~/.codes/.update-state.json.
func stateFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codes", ".update-state.json"), nil
}

// loadState reads the persisted update state.
func loadState() (UpdateState, error) {
	var s UpdateState
	path, err := stateFilePath()
	if err != nil {
		return s, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s, nil // missing file is fine
	}
	_ = json.Unmarshal(data, &s)
	return s, nil
}

// saveState persists the update state.
func saveState(s UpdateState) {
	path, err := stateFilePath()
	if err != nil {
		return
	}
	data, _ := json.Marshal(s)
	_ = os.WriteFile(path, data, 0644)
}

// ShouldCheck reports whether enough time has passed since the last check.
func ShouldCheck(s UpdateState) bool {
	if s.LastCheck == 0 {
		return true
	}
	return time.Since(time.Unix(s.LastCheck, 0)) >= CheckInterval
}

// CheckLatestVersion queries the GitHub API for the latest release.
func CheckLatestVersion() (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// CompareVersions returns true if available is newer than current.
// Both are expected as semver strings like "v1.2.3" or "1.2.3".
func CompareVersions(current, available string) bool {
	cur := parseVersion(current)
	avail := parseVersion(available)
	if cur == nil || avail == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if avail[i] > cur[i] {
			return true
		}
		if avail[i] < cur[i] {
			return false
		}
	}
	return false
}

// parseVersion extracts [major, minor, patch] from a version string.
func parseVersion(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, "-", 2) // strip pre-release suffix
	fields := strings.Split(parts[0], ".")
	if len(fields) != 3 {
		return nil
	}
	result := make([]int, 3)
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil {
			return nil
		}
		result[i] = n
	}
	return result
}

// AutoCheck performs a background update check based on mode.
//
// mode: "notify" prints a message to stderr if a new version is available.
//
//	"silent" downloads the new binary to the staging directory.
//	"off"    does nothing.
func AutoCheck(currentVer, mode string) {
	if mode == "off" || currentVer == "dev" {
		return
	}

	state, _ := loadState()

	// Use cached result if fresh enough
	if !ShouldCheck(state) {
		if state.LatestVersion != "" && CompareVersions(currentVer, state.LatestVersion) {
			if mode == "notify" {
				printNotify(currentVer, state.LatestVersion)
			}
		}
		return
	}

	release, err := CheckLatestVersion()
	if err != nil {
		return // silently ignore network errors in background
	}

	// Update state
	state.LastCheck = time.Now().Unix()
	state.LatestVersion = release.TagName
	saveState(state)

	if !CompareVersions(currentVer, release.TagName) {
		return
	}

	switch mode {
	case "notify":
		printNotify(currentVer, release.TagName)
	case "silent":
		stagingDir, err := stagingDirPath()
		if err != nil {
			return
		}
		_ = os.MkdirAll(stagingDir, 0755)
		_, _ = DownloadRelease(release, stagingDir)
	}
}

func printNotify(current, available string) {
	fmt.Fprintf(os.Stderr, "\nℹ New version available: %s → %s. Run 'codes update' to upgrade.\n\n", current, available)
}
