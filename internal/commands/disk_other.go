//go:build !linux && !darwin && !freebsd && !windows

package commands

import "fmt"

type diskUsage struct {
	Available   uint64
	Total       uint64
	UsedPercent float64
}

func getDiskUsage(path string) (*diskUsage, error) {
	return nil, fmt.Errorf("disk usage check not supported on this platform")
}
