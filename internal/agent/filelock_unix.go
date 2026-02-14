//go:build !windows

package agent

import (
	"os"
	"syscall"
)

// Lock acquires an exclusive file lock (blocking).
func (fl *FileLock) Lock() error {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	fl.f = f
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	if fl.f == nil {
		return nil
	}
	f := fl.f
	fl.f = nil
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return f.Close()
}
