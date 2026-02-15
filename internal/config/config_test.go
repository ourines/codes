package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestAPIConfig_UnmarshalJSON_Migration tests backward compatibility with old flat format.
func TestAPIConfig_UnmarshalJSON_Migration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected APIConfig
	}{
		{
			name: "new format with env map",
			input: `{
				"name": "test",
				"env": {
					"ANTHROPIC_BASE_URL": "https://api.example.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-123"
				}
			}`,
			expected: APIConfig{
				Name: "test",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   "https://api.example.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
				},
			},
		},
		{
			name: "old flat format",
			input: `{
				"name": "test",
				"ANTHROPIC_BASE_URL": "https://api.example.com",
				"ANTHROPIC_AUTH_TOKEN": "sk-test-123"
			}`,
			expected: APIConfig{
				Name: "test",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   "https://api.example.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
				},
			},
		},
		{
			name: "mixed format - env takes precedence",
			input: `{
				"name": "test",
				"env": {
					"ANTHROPIC_BASE_URL": "https://api.new.com"
				},
				"ANTHROPIC_BASE_URL": "https://api.old.com",
				"ANTHROPIC_AUTH_TOKEN": "sk-test-123"
			}`,
			expected: APIConfig{
				Name: "test",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   "https://api.new.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
				},
			},
		},
		{
			name: "empty config",
			input: `{
				"name": "test"
			}`,
			expected: APIConfig{
				Name: "test",
				Env:  map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config APIConfig
			if err := json.Unmarshal([]byte(tt.input), &config); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if config.Name != tt.expected.Name {
				t.Errorf("Name = %q, want %q", config.Name, tt.expected.Name)
			}

			if len(config.Env) != len(tt.expected.Env) {
				t.Errorf("Env length = %d, want %d", len(config.Env), len(tt.expected.Env))
			}

			for key, expectedValue := range tt.expected.Env {
				if actualValue, exists := config.Env[key]; !exists {
					t.Errorf("Env missing key %q", key)
				} else if actualValue != expectedValue {
					t.Errorf("Env[%q] = %q, want %q", key, actualValue, expectedValue)
				}
			}
		})
	}
}

// TestConfig_UnmarshalJSON_Migration tests backward compatibility with old "configs" field.
func TestConfig_UnmarshalJSON_Migration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // expected number of profiles
	}{
		{
			name: "new format with profiles",
			input: `{
				"profiles": [
					{"name": "test1", "env": {"ANTHROPIC_BASE_URL": "https://api1.com"}},
					{"name": "test2", "env": {"ANTHROPIC_BASE_URL": "https://api2.com"}}
				],
				"default": "test1"
			}`,
			expected: 2,
		},
		{
			name: "old format with configs",
			input: `{
				"configs": [
					{"name": "test1", "env": {"ANTHROPIC_BASE_URL": "https://api1.com"}},
					{"name": "test2", "env": {"ANTHROPIC_BASE_URL": "https://api2.com"}}
				],
				"default": "test1"
			}`,
			expected: 2,
		},
		{
			name: "both present - profiles takes precedence",
			input: `{
				"profiles": [
					{"name": "test1", "env": {"ANTHROPIC_BASE_URL": "https://api1.com"}}
				],
				"configs": [
					{"name": "test2", "env": {"ANTHROPIC_BASE_URL": "https://api2.com"}},
					{"name": "test3", "env": {"ANTHROPIC_BASE_URL": "https://api3.com"}}
				],
				"default": "test1"
			}`,
			expected: 1, // profiles takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Config
			if err := json.Unmarshal([]byte(tt.input), &config); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if len(config.Profiles) != tt.expected {
				t.Errorf("Profiles length = %d, want %d", len(config.Profiles), tt.expected)
			}
		})
	}
}

// TestProjectEntry_UnmarshalJSON tests backward compatibility with string format.
func TestProjectEntry_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ProjectEntry
	}{
		{
			name:  "old string format",
			input: `"/path/to/project"`,
			expected: ProjectEntry{
				Path:   "/path/to/project",
				Remote: "",
			},
		},
		{
			name: "new object format - local",
			input: `{
				"path": "/path/to/project"
			}`,
			expected: ProjectEntry{
				Path:   "/path/to/project",
				Remote: "",
			},
		},
		{
			name: "new object format - remote",
			input: `{
				"path": "/remote/path",
				"remote": "hk-server"
			}`,
			expected: ProjectEntry{
				Path:   "/remote/path",
				Remote: "hk-server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var entry ProjectEntry
			if err := json.Unmarshal([]byte(tt.input), &entry); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if entry.Path != tt.expected.Path {
				t.Errorf("Path = %q, want %q", entry.Path, tt.expected.Path)
			}

			if entry.Remote != tt.expected.Remote {
				t.Errorf("Remote = %q, want %q", entry.Remote, tt.expected.Remote)
			}
		})
	}
}

// TestProjectEntry_MarshalJSON tests backward compatible serialization.
func TestProjectEntry_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		entry    ProjectEntry
		expected string
	}{
		{
			name: "local project - serialized as string",
			entry: ProjectEntry{
				Path:   "/path/to/project",
				Remote: "",
			},
			expected: `"/path/to/project"`,
		},
		{
			name: "remote project - serialized as object",
			entry: ProjectEntry{
				Path:   "/remote/path",
				Remote: "hk-server",
			},
			expected: `{"path":"/remote/path","remote":"hk-server"}`,
		},
		{
			name: "project with links - serialized as object",
			entry: ProjectEntry{
				Path: "/path/to/project",
				Links: []ProjectLink{
					{Name: "linked"},
				},
			},
			expected: `{"path":"/path/to/project","links":[{"name":"linked"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.entry)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Marshal = %q, want %q", string(data), tt.expected)
			}
		})
	}
}

// TestAPIConfig_TestAPIConfig tests various HTTP response scenarios.
func TestAPIConfig_TestAPIConfig(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   string
		expected   bool
	}{
		{
			name:       "200 OK with valid response",
			statusCode: http.StatusOK,
			response:   `{"content": [{"text": "Hello"}]}`,
			expected:   true,
		},
		{
			name:       "200 OK with empty response",
			statusCode: http.StatusOK,
			response:   `{}`,
			expected:   true,
		},
		{
			name:       "401 Unauthorized",
			statusCode: http.StatusUnauthorized,
			response:   `{"error": "unauthorized"}`,
			expected:   true, // API reachable but auth failed
		},
		{
			name:       "400 Bad Request",
			statusCode: http.StatusBadRequest,
			response:   `{"error": "bad request"}`,
			expected:   true, // API reachable but request invalid
		},
		{
			name:       "500 Internal Server Error",
			statusCode: http.StatusInternalServerError,
			response:   `{"error": "server error"}`,
			expected:   false,
		},
		{
			name:       "503 Service Unavailable",
			statusCode: http.StatusServiceUnavailable,
			response:   `{"error": "service unavailable"}`,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			// Create test config
			config := APIConfig{
				Name: "test",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   server.URL,
					"ANTHROPIC_AUTH_TOKEN": "sk-test-123",
				},
			}

			result := TestAPIConfig(config)
			if result != tt.expected {
				t.Errorf("TestAPIConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestShouldSkipPermissions tests permission skip logic priority.
func TestShouldSkipPermissions(t *testing.T) {
	tests := []struct {
		name           string
		globalSkip     bool
		profileSkip    *bool
		expectedResult bool
	}{
		{
			name:           "profile override - true",
			globalSkip:     false,
			profileSkip:    boolPtr(true),
			expectedResult: true,
		},
		{
			name:           "profile override - false",
			globalSkip:     true,
			profileSkip:    boolPtr(false),
			expectedResult: false,
		},
		{
			name:           "use global - true",
			globalSkip:     true,
			profileSkip:    nil,
			expectedResult: true,
		},
		{
			name:           "use global - false",
			globalSkip:     false,
			profileSkip:    nil,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				SkipPermissions: tt.globalSkip,
			}
			apiConfig := &APIConfig{
				Name:            "test",
				SkipPermissions: tt.profileSkip,
			}

			result := ShouldSkipPermissionsWithConfig(apiConfig, config)
			if result != tt.expectedResult {
				t.Errorf("ShouldSkipPermissionsWithConfig() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

// TestGetDefaultBehavior tests default behavior validation and fallback.
func TestGetDefaultBehavior(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Save original and restore after test
	origPath := ConfigPath
	ConfigPath = configPath
	defer func() { ConfigPath = origPath }()

	tests := []struct {
		name            string
		configValue     string
		expectedDefault string
	}{
		{
			name:            "valid - current",
			configValue:     "current",
			expectedDefault: "current",
		},
		{
			name:            "valid - last",
			configValue:     "last",
			expectedDefault: "last",
		},
		{
			name:            "valid - home",
			configValue:     "home",
			expectedDefault: "home",
		},
		{
			name:            "invalid value - fallback to current",
			configValue:     "invalid",
			expectedDefault: "current",
		},
		{
			name:            "empty value - fallback to current",
			configValue:     "",
			expectedDefault: "current",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with test value
			config := &Config{
				DefaultBehavior: tt.configValue,
			}
			if err := SaveConfig(config); err != nil {
				t.Fatalf("SaveConfig failed: %v", err)
			}

			result := GetDefaultBehavior()
			if result != tt.expectedDefault {
				t.Errorf("GetDefaultBehavior() = %q, want %q", result, tt.expectedDefault)
			}
		})
	}
}

// TestLoadConfig_SaveConfig_RoundTrip tests config persistence.
func TestLoadConfig_SaveConfig_RoundTrip(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Save original and restore after test
	origPath := ConfigPath
	ConfigPath = configPath
	defer func() { ConfigPath = origPath }()

	// Create test config
	original := &Config{
		Profiles: []APIConfig{
			{
				Name: "test1",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   "https://api1.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-1",
				},
				SkipPermissions: boolPtr(true),
			},
			{
				Name: "test2",
				Env: map[string]string{
					"ANTHROPIC_BASE_URL":   "https://api2.com",
					"ANTHROPIC_AUTH_TOKEN": "sk-test-2",
				},
			},
		},
		Default:         "test1",
		SkipPermissions: false,
		Projects: map[string]ProjectEntry{
			"project1": {Path: "/path/to/project1"},
			"project2": {Path: "/path/to/project2", Remote: "hk"},
		},
		LastWorkDir:     "/tmp/workdir",
		DefaultBehavior: "last",
		Terminal:        "iterm",
		Remotes: []RemoteHost{
			{Name: "hk", Host: "hk.example.com", User: "user", Port: 22},
		},
	}

	// Save config
	if err := SaveConfig(original); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Load config
	loaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify all fields
	if len(loaded.Profiles) != len(original.Profiles) {
		t.Errorf("Profiles length = %d, want %d", len(loaded.Profiles), len(original.Profiles))
	}

	if loaded.Default != original.Default {
		t.Errorf("Default = %q, want %q", loaded.Default, original.Default)
	}

	if loaded.SkipPermissions != original.SkipPermissions {
		t.Errorf("SkipPermissions = %v, want %v", loaded.SkipPermissions, original.SkipPermissions)
	}

	if len(loaded.Projects) != len(original.Projects) {
		t.Errorf("Projects length = %d, want %d", len(loaded.Projects), len(original.Projects))
	}

	if loaded.LastWorkDir != original.LastWorkDir {
		t.Errorf("LastWorkDir = %q, want %q", loaded.LastWorkDir, original.LastWorkDir)
	}

	if loaded.DefaultBehavior != original.DefaultBehavior {
		t.Errorf("DefaultBehavior = %q, want %q", loaded.DefaultBehavior, original.DefaultBehavior)
	}

	if loaded.Terminal != original.Terminal {
		t.Errorf("Terminal = %q, want %q", loaded.Terminal, original.Terminal)
	}

	if len(loaded.Remotes) != len(original.Remotes) {
		t.Errorf("Remotes length = %d, want %d", len(loaded.Remotes), len(original.Remotes))
	}
}

// TestRemoteHost_UserAtHost tests SSH connection string formatting.
func TestRemoteHost_UserAtHost(t *testing.T) {
	tests := []struct {
		name     string
		host     RemoteHost
		expected string
	}{
		{
			name:     "with user",
			host:     RemoteHost{Name: "test", Host: "example.com", User: "ubuntu"},
			expected: "ubuntu@example.com",
		},
		{
			name:     "without user",
			host:     RemoteHost{Name: "test", Host: "example.com"},
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.host.UserAtHost()
			if result != tt.expected {
				t.Errorf("UserAtHost() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// Helper function to create bool pointer.
func boolPtr(b bool) *bool {
	return &b
}

