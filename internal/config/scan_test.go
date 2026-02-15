package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeClaudeProjectPath(t *testing.T) {
	// Create temp directories to simulate real filesystem paths
	tmpDir := t.TempDir()

	// Create test directory structures
	testDirs := []struct {
		path    string
		encoded string
	}{
		{
			path:    filepath.Join(tmpDir, "Users", "test", "Projects", "myapp"),
			encoded: "-" + pathToEncoded(filepath.Join(tmpDir, "Users", "test", "Projects", "myapp")),
		},
		{
			path:    filepath.Join(tmpDir, "Users", "test", "code", "my-project"),
			encoded: "-" + pathToEncoded(filepath.Join(tmpDir, "Users", "test", "code", "my-project")),
		},
		{
			path:    filepath.Join(tmpDir, "Users", "test", "deep", "nested", "path"),
			encoded: "-" + pathToEncoded(filepath.Join(tmpDir, "Users", "test", "deep", "nested", "path")),
		},
	}

	for _, td := range testDirs {
		if err := os.MkdirAll(td.path, 0o755); err != nil {
			t.Fatalf("failed to create test dir %s: %v", td.path, err)
		}
	}

	for _, tc := range testDirs {
		t.Run(tc.encoded, func(t *testing.T) {
			result := decodeClaudeProjectPath(tc.encoded)
			if result != tc.path {
				t.Errorf("decodeClaudeProjectPath(%q)\n  got  %q\n  want %q", tc.encoded, result, tc.path)
			}
		})
	}
}

func TestDecodeClaudeProjectPathWithHyphen(t *testing.T) {
	tmpDir := t.TempDir()

	// Create path with hyphen in directory name: /tmp/xxx/code/my-project
	projectPath := filepath.Join(tmpDir, "code", "my-project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatal(err)
	}

	// Encoded: -<tmpDir parts>-code-my-project
	// The challenge is "my-project" has a hyphen
	encoded := "-" + pathToEncoded(projectPath)

	result := decodeClaudeProjectPath(encoded)
	if result != projectPath {
		t.Errorf("decodeClaudeProjectPath(%q)\n  got  %q\n  want %q", encoded, result, projectPath)
	}
}

func TestDecodeClaudeProjectPathInvalid(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
	}{
		{"empty string", "", ""},
		{"no leading dash", "Users-test", ""},
		{"nonexistent path", "-nonexistent-path-xyz-abc", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := decodeClaudeProjectPath(tc.encoded)
			if result != tc.want {
				t.Errorf("got %q, want %q", result, tc.want)
			}
		})
	}
}

func TestUniqueAlias(t *testing.T) {
	existing := map[string]ProjectEntry{
		"codes": {Path: "/a"},
		"myapp": {Path: "/b"},
	}

	tests := []struct {
		base string
		want string
	}{
		{"codes", "codes-2"},
		{"myapp", "myapp-2"},
		{"newproject", "newproject"},
	}

	for _, tc := range tests {
		t.Run(tc.base, func(t *testing.T) {
			got := uniqueAlias(tc.base, existing)
			if got != tc.want {
				t.Errorf("uniqueAlias(%q) = %q, want %q", tc.base, got, tc.want)
			}
		})
	}

	// Test sequential suffixes
	existing["codes-2"] = ProjectEntry{Path: "/c"}
	got := uniqueAlias("codes", existing)
	if got != "codes-3" {
		t.Errorf("uniqueAlias with codes-2 taken: got %q, want %q", got, "codes-3")
	}
}

func TestImportDiscoveredProjects(t *testing.T) {
	// Set up a temporary config
	tmpDir := t.TempDir()
	origConfigPath := ConfigPath
	ConfigPath = filepath.Join(tmpDir, "config.json")
	defer func() { ConfigPath = origConfigPath }()

	// Create initial config with one existing project
	cfg := &Config{
		Projects: map[string]ProjectEntry{
			"existing": {Path: "/existing/path"},
		},
	}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}

	projects := []DiscoveredProject{
		{Path: "/existing/path", Name: "existing"},   // Should be skipped (same path)
		{Path: "/new/project/one", Name: "one"},       // Should be added
		{Path: "/new/project/two", Name: "two"},       // Should be added
		{Path: "/new/project/existing", Name: "existing"}, // Should be added with suffix
	}

	added, skipped, err := ImportDiscoveredProjects(projects)
	if err != nil {
		t.Fatalf("ImportDiscoveredProjects error: %v", err)
	}

	if added != 3 {
		t.Errorf("added = %d, want 3", added)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}

	// Verify the config was saved correctly
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Projects) != 4 {
		t.Errorf("expected 4 projects, got %d", len(cfg.Projects))
	}

	// Check the alias conflict was resolved
	if _, ok := cfg.Projects["existing-2"]; !ok {
		t.Error("expected 'existing-2' alias for conflicting project")
	}
}

// pathToEncoded converts a real path to Claude's encoding by replacing "/" with "-"
// and stripping the leading "/".
func pathToEncoded(path string) string {
	// Normalize to forward slashes for cross-platform consistency
	path = filepath.ToSlash(path)
	// Remove leading "/"
	if len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}
	return strings.ReplaceAll(path, "/", "-")
}
