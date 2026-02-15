//go:build windows

package commands

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type diskUsage struct {
	Available   uint64
	Total       uint64
	UsedPercent float64
}

func getDiskUsage(path string) (*diskUsage, error) {
	// Extract drive letter, handling edge cases
	if len(path) < 2 || path[1] != ':' {
		// Not a standard drive path (e.g., UNC path, relative path)
		// Return zero usage gracefully
		return &diskUsage{
			Available:   0,
			Total:       0,
			UsedPercent: 0,
		}, nil
	}

	drive := path[:1] // e.g. "C"

	// Use PowerShell with null checks to handle network drives and unmounted drives
	cmd := fmt.Sprintf(
		"$d = Get-PSDrive %s -ErrorAction SilentlyContinue; "+
			"if ($d -and $d.Free -ne $null -and $d.Used -ne $null) { "+
			"Write-Output $d.Free; Write-Output $d.Used "+
			"} else { "+
			"Write-Output 0; Write-Output 0 "+
			"}",
		drive,
	)

	out, err := exec.Command("powershell", "-NoProfile", "-Command", cmd).Output()
	if err != nil {
		// Gracefully handle PowerShell failures (e.g., restricted execution policy)
		return &diskUsage{
			Available:   0,
			Total:       0,
			UsedPercent: 0,
		}, nil
	}

	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) < 2 {
		// Unexpected output, return zero usage
		return &diskUsage{
			Available:   0,
			Total:       0,
			UsedPercent: 0,
		}, nil
	}

	free, err := strconv.ParseUint(lines[0], 10, 64)
	if err != nil {
		// Parse failure (e.g., non-numeric output), return zero
		return &diskUsage{
			Available:   0,
			Total:       0,
			UsedPercent: 0,
		}, nil
	}

	used, err := strconv.ParseUint(lines[1], 10, 64)
	if err != nil {
		// Parse failure, return zero
		return &diskUsage{
			Available:   0,
			Total:       0,
			UsedPercent: 0,
		}, nil
	}

	total := free + used
	var pct float64
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	return &diskUsage{
		Available:   free,
		Total:       total,
		UsedPercent: pct,
	}, nil
}
