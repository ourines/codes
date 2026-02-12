package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// statusCache is the on-disk format for cached remote status.
type statusCache struct {
	Hosts map[string]*RemoteStatus `json:"hosts"`
}

var cachePath string
var cacheOnce sync.Once

func getCachePath() string {
	cacheOnce.Do(func() {
		homeDir, _ := os.UserHomeDir()
		cachePath = filepath.Join(homeDir, ".codes", "remote-status.json")
	})
	return cachePath
}

// LoadStatusCache loads cached remote status from disk.
// Returns an empty map if the file doesn't exist or is invalid.
func LoadStatusCache() map[string]*RemoteStatus {
	data, err := os.ReadFile(getCachePath())
	if err != nil {
		return make(map[string]*RemoteStatus)
	}

	var cache statusCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]*RemoteStatus)
	}

	if cache.Hosts == nil {
		return make(map[string]*RemoteStatus)
	}
	return cache.Hosts
}

// SaveStatusCache writes the full status cache to disk.
func SaveStatusCache(hosts map[string]*RemoteStatus) error {
	cache := statusCache{Hosts: hosts}
	data, err := json.MarshalIndent(cache, "", "    ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(getCachePath())
	os.MkdirAll(dir, 0755)
	return os.WriteFile(getCachePath(), data, 0644)
}

// UpdateStatusCache updates a single host's status and saves to disk.
func UpdateStatusCache(name string, status *RemoteStatus) error {
	cache := LoadStatusCache()
	cache[name] = status
	return SaveStatusCache(cache)
}

// DeleteStatusCache removes a host from the cache and saves to disk.
func DeleteStatusCache(name string) error {
	cache := LoadStatusCache()
	delete(cache, name)
	return SaveStatusCache(cache)
}
