//go:build windows

package agent

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock = 0x00000002
)

// Lock acquires an exclusive file lock (blocking).
func (fl *FileLock) Lock() error {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	fl.f = f

	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(
		uintptr(f.Fd()),
		uintptr(lockfileExclusiveLock),
		0,
		1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r1 == 0 {
		return err
	}
	return nil
}

// Unlock releases the file lock.
func (fl *FileLock) Unlock() error {
	if fl.f == nil {
		return nil
	}
	f := fl.f
	fl.f = nil

	var ol syscall.Overlapped
	procUnlockFileEx.Call(
		uintptr(f.Fd()),
		0,
		1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	return f.Close()
}
