package session

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Checkpoint represents a git snapshot point.
type Checkpoint struct {
	HeadHash  string    // git HEAD hash at checkpoint time
	StashRef  string    // git stash create ref (empty string if working tree was clean)
	Dir       string    // project directory
	CreatedAt time.Time
}

// DiffFile represents change stats for a single file.
type DiffFile struct {
	Path      string
	Additions int
	Deletions int
	Status    string // "M"=modified, "A"=added, "D"=deleted, "R"=renamed
}

// DiffSummary represents the full diff statistics.
type DiffSummary struct {
	Files      []DiffFile
	TotalAdded int
	TotalDel   int
}

// CreateCheckpoint captures the current git state as a checkpoint.
func CreateCheckpoint(dir string) (*Checkpoint, error) {
	// Get current HEAD hash
	headHash, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("git rev-parse HEAD: %w", err)
	}

	// Create stash without applying it (captures working tree + index changes)
	stashRef, _ := gitOutput(dir, "stash", "create")
	// stash create returns empty if working tree is clean — that's fine

	return &Checkpoint{
		HeadHash:  headHash,
		StashRef:  stashRef,
		Dir:       dir,
		CreatedAt: time.Now(),
	}, nil
}

// GetDiffSummary computes the diff between the checkpoint HEAD and current working tree.
func GetDiffSummary(dir string, cp *Checkpoint) (*DiffSummary, error) {
	summary := &DiffSummary{}

	// Get numstat (additions/deletions per file)
	numstatOut, err := gitOutput(dir, "diff", "--numstat", cp.HeadHash)
	if err != nil {
		// Also try against HEAD for unstaged changes
		numstatOut, err = gitOutput(dir, "diff", "--numstat", "HEAD")
		if err != nil {
			return summary, nil // no changes
		}
	}

	// Get name-status (status letter per file)
	statusOut, _ := gitOutput(dir, "diff", "--name-status", cp.HeadHash)

	// Parse status map
	statusMap := make(map[string]string)
	for _, line := range splitLines(statusOut) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			statusMap[parts[1]] = parts[0]
		}
	}

	// Also include untracked files
	untrackedOut, _ := gitOutput(dir, "ls-files", "--others", "--exclude-standard")
	for _, f := range splitLines(untrackedOut) {
		if f != "" {
			statusMap[f] = "A"
		}
	}

	// Parse numstat
	for _, line := range splitLines(numstatOut) {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		path := parts[2]

		status := statusMap[path]
		if status == "" {
			status = "M"
		}

		summary.Files = append(summary.Files, DiffFile{
			Path:      path,
			Additions: added,
			Deletions: deleted,
			Status:    status,
		})
		summary.TotalAdded += added
		summary.TotalDel += deleted
	}

	// Add untracked files that weren't in numstat
	seen := make(map[string]bool)
	for _, f := range summary.Files {
		seen[f.Path] = true
	}
	for _, f := range splitLines(untrackedOut) {
		if f != "" && !seen[f] {
			summary.Files = append(summary.Files, DiffFile{
				Path:   f,
				Status: "A",
			})
		}
	}

	return summary, nil
}

// RollbackAll reverts the working tree to the checkpoint state.
func RollbackAll(dir string, cp *Checkpoint) error {
	// Restore tracked files to checkpoint HEAD
	if _, err := gitOutput(dir, "checkout", cp.HeadHash, "--", "."); err != nil {
		return fmt.Errorf("git checkout: %w", err)
	}

	// Remove untracked files
	if _, err := gitOutput(dir, "clean", "-fd"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}

	return nil
}

// RollbackFiles selectively reverts specific files to the checkpoint state.
func RollbackFiles(dir string, cp *Checkpoint, files []string) error {
	for _, f := range files {
		// Check if the file existed at checkpoint HEAD
		_, err := gitOutput(dir, "cat-file", "-e", cp.HeadHash+":"+f)
		if err != nil {
			// File didn't exist at checkpoint — it's a new file, remove it
			os.Remove(dir + "/" + f)
			continue
		}
		// File existed — restore it
		if _, err := gitOutput(dir, "checkout", cp.HeadHash, "--", f); err != nil {
			return fmt.Errorf("rollback %s: %w", f, err)
		}
	}
	return nil
}

// gitOutput runs a git command in the given directory and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// splitLines splits a string into non-empty lines.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	result := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			result = append(result, l)
		}
	}
	return result
}
