package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codes/internal/config"
)

// claudeProjectsDir returns the path to ~/.claude/projects/.
func claudeProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// projectPathFromDir converts a Claude project directory name back to an absolute path.
// e.g. "-Users-ourines-Projects-codes" -> "/Users/ourines/Projects/codes"
func projectPathFromDir(dirName string) string {
	// The directory name has leading "-" and all "/" replaced with "-"
	// We need to reconstruct the path.
	if dirName == "" {
		return ""
	}
	// Split by "-" and reconstruct
	// "-Users-ourines-Projects-codes" -> ["", "Users", "ourines", "Projects", "codes"]
	parts := strings.Split(dirName, "-")
	return "/" + strings.Join(parts[1:], "/")
}

// resolveProjectAlias looks up the project alias from the codes config.
// Returns the alias if found, otherwise the raw path.
func resolveProjectAlias(projectPath string, projects map[string]config.ProjectEntry) string {
	for name, entry := range projects {
		if entry.Path == projectPath {
			return name
		}
	}
	// Fall back to last path component
	return filepath.Base(projectPath)
}

// jsonlLine represents a single line from a Claude session JSONL file.
// We only parse the fields we need.
type jsonlLine struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   *struct {
		Model string `json:"model"`
		Usage *struct {
			InputTokens       int64 `json:"input_tokens"`
			OutputTokens      int64 `json:"output_tokens"`
			CacheCreateTokens int64 `json:"cache_creation_input_tokens"`
			CacheReadTokens   int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ScanOptions controls scanner behavior.
type ScanOptions struct {
	// Since only scans files modified after this time (for incremental scanning).
	// Zero value means scan everything.
	Since time.Time
}

// ScanSessions scans all Claude session JSONL files and returns SessionRecords.
func ScanSessions(opts ScanOptions) ([]SessionRecord, error) {
	projDir, err := claudeProjectsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(projDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	// Load codes project map for alias resolution
	projects, _ := config.ListProjects()
	if projects == nil {
		projects = make(map[string]config.ProjectEntry)
	}

	var records []SessionRecord

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectPath := projectPathFromDir(entry.Name())
		projectAlias := resolveProjectAlias(projectPath, projects)
		dirPath := filepath.Join(projDir, entry.Name())

		sessionFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if !strings.HasSuffix(sf.Name(), ".jsonl") {
				continue
			}

			// Incremental: skip files not modified since last scan
			if !opts.Since.IsZero() {
				info, err := sf.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(opts.Since) {
					continue
				}
			}

			sessionID := strings.TrimSuffix(sf.Name(), ".jsonl")
			filePath := filepath.Join(dirPath, sf.Name())

			record, err := parseSessionFile(filePath, sessionID, projectAlias, projectPath)
			if err != nil {
				continue // skip unparseable files
			}
			if record != nil {
				records = append(records, *record)
			}
		}
	}

	return records, nil
}

// parseSessionFile reads a single JSONL session file and extracts a SessionRecord.
func parseSessionFile(path, sessionID, project, projectPath string) (*SessionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	record := &SessionRecord{
		SessionID:   sessionID,
		Project:     project,
		ProjectPath: projectPath,
	}

	var (
		turns     int
		firstTime time.Time
		lastTime  time.Time
		mainModel string
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for long lines

	for scanner.Scan() {
		var line jsonlLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		// Track timestamps from any line type
		if !line.Timestamp.IsZero() {
			if firstTime.IsZero() || line.Timestamp.Before(firstTime) {
				firstTime = line.Timestamp
			}
			if line.Timestamp.After(lastTime) {
				lastTime = line.Timestamp
			}
		}

		if line.Type != "assistant" {
			continue
		}
		if line.Message == nil {
			continue
		}

		turns++

		if line.Message.Model != "" {
			mainModel = line.Message.Model
		}

		if line.Message.Usage != nil {
			u := line.Message.Usage
			record.InputTokens += u.InputTokens
			record.OutputTokens += u.OutputTokens
			record.CacheCreateTokens += u.CacheCreateTokens
			record.CacheReadTokens += u.CacheReadTokens
		}
	}

	// Skip sessions with no assistant messages
	if turns == 0 {
		return nil, nil
	}

	record.Model = mainModel
	record.Turns = turns
	record.StartTime = firstTime
	record.EndTime = lastTime
	if !firstTime.IsZero() && !lastTime.IsZero() {
		record.Duration = lastTime.Sub(firstTime)
	}

	// Calculate cost
	record.CostUSD = CalculateCost(mainModel, Usage{
		InputTokens:       record.InputTokens,
		OutputTokens:      record.OutputTokens,
		CacheCreateTokens: record.CacheCreateTokens,
		CacheReadTokens:   record.CacheReadTokens,
	})

	return record, nil
}
