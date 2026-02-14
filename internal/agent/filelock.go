package agent

import "os"

// FileLock provides mutual exclusion via file-based locking.
// Platform-specific implementations are in filelock_unix.go and filelock_windows.go.
type FileLock struct {
	path string
	f    *os.File
}

// NewFileLock creates a new file lock for the given path.
func NewFileLock(path string) *FileLock {
	return &FileLock{path: path}
}

// Lock and Unlock are implemented in platform-specific files.
