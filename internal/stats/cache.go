package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// cacheFileName is the stats cache file under ~/.codes/
	cacheFileName = "stats.json"
	// refreshInterval is the minimum time between automatic rescans.
	refreshInterval = 5 * time.Minute
)

// cachePath returns the full path to the stats cache file.
func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".codes", cacheFileName), nil
}

// LoadCache reads the stats cache from disk.
// Returns an empty cache (not an error) if the file doesn't exist.
func LoadCache() (*StatsCache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &StatsCache{}, nil
		}
		return nil, fmt.Errorf("read stats cache: %w", err)
	}

	var cache StatsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		// Corrupted cache — start fresh
		return &StatsCache{}, nil
	}

	return &cache, nil
}

// SaveCache writes the stats cache to disk.
func SaveCache(cache *StatsCache) error {
	path, err := cachePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal stats cache: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	// Atomic write via temp file + rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write stats cache: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename stats cache: %w", err)
	}

	return nil
}

// RefreshIfNeeded rescans session data if the cache is older than refreshInterval.
// Returns the (possibly updated) cache.
func RefreshIfNeeded(cache *StatsCache) (*StatsCache, error) {
	if cache == nil {
		cache = &StatsCache{}
	}

	if time.Since(cache.LastScan) < refreshInterval {
		return cache, nil
	}

	return ForceRefresh(cache)
}

// ForceRefresh performs a full rescan regardless of cache age.
func ForceRefresh(cache *StatsCache) (*StatsCache, error) {
	if cache == nil {
		cache = &StatsCache{}
	}

	records, err := ScanSessions(ScanOptions{})
	if err != nil {
		return cache, fmt.Errorf("scan sessions: %w", err)
	}

	cache.Sessions = records
	cache.DailyStats = Aggregate(records, time.Time{}, time.Time{})
	cache.LastScan = time.Now()

	if err := SaveCache(cache); err != nil {
		return cache, fmt.Errorf("save cache: %w", err)
	}

	return cache, nil
}
