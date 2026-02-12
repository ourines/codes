package update

import "time"

// CheckInterval is the minimum duration between automatic update checks.
const CheckInterval = 24 * time.Hour

// ReleaseInfo holds metadata about a GitHub release.
type ReleaseInfo struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
}

// UpdateState persists the last check timestamp and latest known version.
// Stored at ~/.codes/.update-state.json, separate from config.json to avoid races.
type UpdateState struct {
	LastCheck     int64  `json:"last_check"`
	LatestVersion string `json:"latest_version"`
}
