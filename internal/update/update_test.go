package update

import (
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current   string
		available string
		want      bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.0", "v1.1.0", true},
		{"v1.0.0", "v2.0.0", true},
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.3", "v1.2.2", false},
		{"v2.0.0", "v1.9.9", false},
		{"1.0.0", "1.0.1", true},     // without v prefix
		{"v1.0.0", "1.0.1", true},    // mixed prefix
		{"v1.0.0-rc1", "v1.0.1", true}, // pre-release stripped
		{"invalid", "v1.0.0", false},
		{"v1.0.0", "invalid", false},
		{"v1.0", "v1.0.1", false},    // incomplete version
	}

	for _, tt := range tests {
		t.Run(tt.current+"_vs_"+tt.available, func(t *testing.T) {
			got := CompareVersions(tt.current, tt.available)
			if got != tt.want {
				t.Errorf("CompareVersions(%q, %q) = %v, want %v", tt.current, tt.available, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"v1.2.3", []int{1, 2, 3}},
		{"1.2.3", []int{1, 2, 3}},
		{"v0.0.1", []int{0, 0, 1}},
		{"v1.2.3-rc1", []int{1, 2, 3}},
		{"invalid", nil},
		{"v1.2", nil},
		{"v1.2.3.4", nil},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseVersion(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("parseVersion(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Errorf("parseVersion(%q) = nil, want %v", tt.input, tt.want)
				return
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("parseVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestShouldCheck(t *testing.T) {
	tests := []struct {
		name  string
		state UpdateState
		want  bool
	}{
		{"zero value", UpdateState{}, true},
		{"old timestamp", UpdateState{LastCheck: 1000000}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldCheck(tt.state)
			if got != tt.want {
				t.Errorf("ShouldCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPlatformBinaryName(t *testing.T) {
	name := platformBinaryName()
	if name == "" {
		t.Error("platformBinaryName() returned empty string")
	}
	if runtime.GOOS == "windows" {
		if name != "codes.exe" {
			t.Errorf("platformBinaryName() = %q, want %q", name, "codes.exe")
		}
	} else {
		if name != "codes" {
			t.Errorf("platformBinaryName() = %q, want %q", name, "codes")
		}
	}
}
