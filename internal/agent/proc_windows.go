//go:build windows

package agent

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// setSysProcAttr configures platform-specific process attributes.
// On Windows, we use CREATE_NEW_PROCESS_GROUP so the child process
// can be terminated without affecting the parent.
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	const STILL_ACTIVE = 259

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")

	handle, _, _ := openProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION,
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(handle))

	var exitCode uint32
	getExitCodeProcess := kernel32.NewProc("GetExitCodeProcess")
	ret, _, _ := getExitCodeProcess.Call(uintptr(handle), uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false
	}
	return exitCode == STILL_ACTIVE
}
