package dispatch

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name         string
		userInput    string
		projects     []string
		wantSystem   string // substring to check in system prompt
		wantUser     string
	}{
		{
			name:       "with projects",
			userInput:  "Fix the auth bug in myapp",
			projects:   []string{"myapp", "codes", "frontend"},
			wantSystem: "myapp, codes, frontend",
			wantUser:   "Fix the auth bug in myapp",
		},
		{
			name:       "no projects",
			userInput:  "Do something",
			projects:   nil,
			wantSystem: "None registered",
			wantUser:   "Do something",
		},
		{
			name:       "single project",
			userInput:  "Run tests",
			projects:   []string{"solo"},
			wantSystem: "solo",
			wantUser:   "Run tests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			system, user := buildPrompt(tt.userInput, tt.projects)
			if user != tt.wantUser {
				t.Errorf("user prompt = %q, want %q", user, tt.wantUser)
			}
			if tt.wantSystem != "" && !contains(system, tt.wantSystem) {
				t.Errorf("system prompt does not contain %q\ngot: %s", tt.wantSystem, system)
			}
		})
	}
}

func TestWorkerNames(t *testing.T) {
	names := workerNames(3)
	if len(names) != 3 {
		t.Fatalf("expected 3 workers, got %d", len(names))
	}
	expected := []string{"worker-1", "worker-2", "worker-3"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("worker[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestParseIntentJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*IntentResponse) error
	}{
		{
			name: "clean JSON",
			input: `{"project":"myapp","tasks":[{"subject":"Fix auth","description":"Fix the auth bug","priority":"high"}],"clarify":"","error":""}`,
			check: func(r *IntentResponse) error {
				if r.Project != "myapp" {
					return errorf("project = %q, want %q", r.Project, "myapp")
				}
				if len(r.Tasks) != 1 {
					return errorf("tasks count = %d, want 1", len(r.Tasks))
				}
				if r.Tasks[0].Subject != "Fix auth" {
					return errorf("task subject = %q, want %q", r.Tasks[0].Subject, "Fix auth")
				}
				return nil
			},
		},
		{
			name: "JSON wrapped in markdown",
			input: "Here is the analysis:\n```json\n{\"project\":\"codes\",\"tasks\":[{\"subject\":\"Run tests\",\"description\":\"Execute test suite\"}],\"clarify\":\"\",\"error\":\"\"}\n```",
			check: func(r *IntentResponse) error {
				if r.Project != "codes" {
					return errorf("project = %q, want %q", r.Project, "codes")
				}
				return nil
			},
		},
		{
			name: "clarify response",
			input: `{"project":"","tasks":[],"clarify":"Which project do you mean?","error":""}`,
			check: func(r *IntentResponse) error {
				if r.Clarify != "Which project do you mean?" {
					return errorf("clarify = %q, want question", r.Clarify)
				}
				return nil
			},
		},
		{
			name: "with dependencies",
			input: `{"project":"myapp","tasks":[{"subject":"Write code","description":"Implement feature","dependsOn":[]},{"subject":"Write tests","description":"Add tests","dependsOn":[1]}],"clarify":"","error":""}`,
			check: func(r *IntentResponse) error {
				if len(r.Tasks) != 2 {
					return errorf("tasks count = %d, want 2", len(r.Tasks))
				}
				if len(r.Tasks[1].DependsOn) != 1 || r.Tasks[1].DependsOn[0] != 1 {
					return errorf("dependsOn = %v, want [1]", r.Tasks[1].DependsOn)
				}
				return nil
			},
		},
		{
			name:    "invalid JSON",
			input:   "this is not json at all",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseIntentJSON(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				if err := tt.check(result); err != nil {
					t.Error(err)
				}
			}
		})
	}
}

func TestDispatchOptionsDefaults(t *testing.T) {
	// Verify that Dispatch rejects empty input
	_, err := Dispatch(nil, DispatchOptions{})
	if err == nil {
		t.Fatal("expected error for empty user input")
	}
}

// helpers

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func errorf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
