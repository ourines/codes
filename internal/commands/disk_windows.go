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
	// Use PowerShell to query disk info on Windows.
	drive := path[:2] // e.g. "C:"
	out, err := exec.Command("powershell", "-Command",
		"(Get-PSDrive "+drive[:1]+").Free, (Get-PSDrive "+drive[:1]+").Used").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Fields(strings.TrimSpace(string(out)))
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected powershell output")
	}
	free, err := strconv.ParseUint(lines[0], 10, 64)
	if err != nil {
		return nil, err
	}
	used, err := strconv.ParseUint(lines[1], 10, 64)
	if err != nil {
		return nil, err
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
