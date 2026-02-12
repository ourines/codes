//go:build !windows

package commands

import (
	"testing"
)

func TestCheckExecutionPolicy_NonWindows(t *testing.T) {
	// On non-Windows platforms, execution policy check always passes
	if !checkExecutionPolicy(false) {
		t.Error("checkExecutionPolicy() should return true on non-Windows")
	}
}

func TestEnsureInPath_NonWindows(t *testing.T) {
	// On non-Windows platforms, ensureInPath is a no-op
	if !ensureInPath("/usr/local/bin") {
		t.Error("ensureInPath() should return true on non-Windows")
	}
}

func TestInstallPowerShellCompletion_NonWindows(t *testing.T) {
	// On non-Windows, PowerShell completion is not applicable
	if installPowerShellCompletion() {
		t.Error("installPowerShellCompletion() should return false on non-Windows")
	}
}
