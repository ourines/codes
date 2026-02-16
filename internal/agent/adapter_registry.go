package agent

import (
	"fmt"
	"sync"
)

var (
	adapters = make(map[string]CLIAdapter)
	mu       sync.RWMutex
)

// RegisterAdapter registers a CLI adapter with the given name.
// This is typically called in init() functions of adapter implementations.
func RegisterAdapter(name string, adapter CLIAdapter) {
	mu.Lock()
	defer mu.Unlock()
	adapters[name] = adapter
}

// GetAdapter returns the adapter with the given name.
// Returns an error if the adapter is not registered or not available.
func GetAdapter(name string) (CLIAdapter, error) {
	mu.RLock()
	defer mu.RUnlock()

	adapter, ok := adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter %q not registered", name)
	}

	if !adapter.Available() {
		return nil, fmt.Errorf("adapter %q is registered but not available (CLI tool not found)", name)
	}

	return adapter, nil
}

// ListAdapters returns the names of all registered adapters.
func ListAdapters() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	return names
}

// DefaultAdapter returns the first available adapter, preferring claude.
// Returns nil if no adapters are available.
func DefaultAdapter() CLIAdapter {
	// Try claude first (default behavior)
	if adapter, err := GetAdapter("claude"); err == nil {
		return adapter
	}

	// Fall back to any available adapter
	mu.RLock()
	defer mu.RUnlock()

	for _, adapter := range adapters {
		if adapter.Available() {
			return adapter
		}
	}

	return nil
}
