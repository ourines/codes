package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initTestRepo creates a temporary git repo with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\n%s", args, err, out)
		}
	}

	// Create initial file and commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "initial"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\n%s", args, err, out)
		}
	}

	return dir
}

func TestCreateCheckpointCleanRepo(t *testing.T) {
	dir := initTestRepo(t)

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if cp.HeadHash == "" {
		t.Error("HeadHash should not be empty")
	}
	if cp.StashRef != "" {
		t.Errorf("StashRef should be empty for clean repo, got %q", cp.StashRef)
	}
	if cp.Dir != dir {
		t.Errorf("Dir = %q, want %q", cp.Dir, dir)
	}
}

func TestCreateCheckpointDirtyRepo(t *testing.T) {
	dir := initTestRepo(t)

	// Make a modification
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}

	if cp.HeadHash == "" {
		t.Error("HeadHash should not be empty")
	}
	if cp.StashRef == "" {
		t.Error("StashRef should not be empty for dirty repo")
	}
}

func TestGetDiffSummary(t *testing.T) {
	dir := initTestRepo(t)

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Modify an existing file
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\nNew line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Add a new file
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := GetDiffSummary(dir, cp)
	if err != nil {
		t.Fatalf("GetDiffSummary: %v", err)
	}

	if len(summary.Files) == 0 {
		t.Fatal("expected at least one changed file")
	}

	// Check that README.md is in the diff
	found := false
	for _, f := range summary.Files {
		if f.Path == "README.md" {
			found = true
			if f.Additions == 0 && f.Deletions == 0 {
				t.Error("README.md should have additions or deletions")
			}
		}
	}
	if !found {
		t.Error("README.md not found in diff")
	}

	// Check totals
	if summary.TotalAdded == 0 {
		t.Error("TotalAdded should be > 0")
	}
}

func TestRollbackAll(t *testing.T) {
	dir := initTestRepo(t)

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}

	originalContent, _ := os.ReadFile(filepath.Join(dir, "README.md"))

	// Modify file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\n"), 0644)
	// Add new file
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0644)

	// Rollback
	if err := RollbackAll(dir, cp); err != nil {
		t.Fatalf("RollbackAll: %v", err)
	}

	// Check README.md is restored
	content, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if string(content) != string(originalContent) {
		t.Errorf("README.md content = %q, want %q", content, originalContent)
	}

	// Check new file is removed
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Error("new.txt should have been removed by rollback")
	}
}

func TestRollbackFiles(t *testing.T) {
	dir := initTestRepo(t)

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}

	originalContent, _ := os.ReadFile(filepath.Join(dir, "README.md"))

	// Modify README.md
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Changed\n"), 0644)
	// Add two new files
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("keep\n"), 0644)
	os.WriteFile(filepath.Join(dir, "remove.txt"), []byte("remove\n"), 0644)

	// Rollback only README.md and remove.txt
	if err := RollbackFiles(dir, cp, []string{"README.md", "remove.txt"}); err != nil {
		t.Fatalf("RollbackFiles: %v", err)
	}

	// README.md should be restored
	content, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if string(content) != string(originalContent) {
		t.Errorf("README.md = %q, want %q", content, originalContent)
	}

	// remove.txt should be deleted (didn't exist in checkpoint)
	if _, err := os.Stat(filepath.Join(dir, "remove.txt")); !os.IsNotExist(err) {
		t.Error("remove.txt should have been removed")
	}

	// keep.txt should still exist
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Error("keep.txt should still exist")
	}
}

func TestGetDiffSummaryNoChanges(t *testing.T) {
	dir := initTestRepo(t)

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}

	summary, err := GetDiffSummary(dir, cp)
	if err != nil {
		t.Fatalf("GetDiffSummary: %v", err)
	}

	if len(summary.Files) != 0 {
		t.Errorf("expected no files, got %d", len(summary.Files))
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a\nb\nc", 3},
		{"a\n\nb", 2},
		{"single", 1},
	}

	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestCreateCheckpointNotGitRepo(t *testing.T) {
	dir := t.TempDir()

	_, err := CreateCheckpoint(dir)
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestRollbackFilesDeletedFile(t *testing.T) {
	dir := initTestRepo(t)

	// Create and commit a file
	os.WriteFile(filepath.Join(dir, "to-delete.txt"), []byte("delete me\n"), 0644)
	for _, args := range [][]string{
		{"git", "add", "."},
		{"git", "commit", "-m", "add to-delete"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.CombinedOutput()
	}

	cp, err := CreateCheckpoint(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Delete the file
	os.Remove(filepath.Join(dir, "to-delete.txt"))

	// Verify it's gone
	if _, err := os.Stat(filepath.Join(dir, "to-delete.txt")); !os.IsNotExist(err) {
		t.Fatal("file should be deleted")
	}

	// Rollback should restore it
	if err := RollbackFiles(dir, cp, []string{"to-delete.txt"}); err != nil {
		t.Fatalf("RollbackFiles: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "to-delete.txt"))
	if err != nil {
		t.Fatal("to-delete.txt should be restored")
	}
	if !strings.Contains(string(content), "delete me") {
		t.Errorf("unexpected content: %q", content)
	}
}
