//go:build !windows

package commands

import "syscall"

type diskUsage struct {
	Available   uint64
	Total       uint64
	UsedPercent float64
}

func getDiskUsage(path string) (*diskUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, err
	}
	available := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	used := total - available
	return &diskUsage{
		Available:   available,
		Total:       total,
		UsedPercent: float64(used) / float64(total) * 100,
	}, nil
}
