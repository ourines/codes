package remote

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"codes/internal/config"
)

// sshArgs builds common SSH arguments from a RemoteHost config.
func sshArgs(host *config.RemoteHost) []string {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if host.Port != 0 {
		args = append(args, "-p", fmt.Sprintf("%d", host.Port))
	}
	if host.Identity != "" {
		args = append(args, "-i", expandHome(host.Identity))
	}
	return args
}

// RunSSH executes a command on the remote host and returns stdout.
func RunSSH(host *config.RemoteHost, command string) (string, error) {
	args := sshArgs(host)
	args = append(args, host.UserAtHost(), command)

	cmd := exec.Command("ssh", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ssh %s: %w", host.UserAtHost(), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunSSHInteractive opens an interactive SSH session with TTY allocation.
func RunSSHInteractive(host *config.RemoteHost, command string) error {
	args := []string{"-t"} // force TTY
	args = append(args, sshArgs(host)...)
	args = append(args, host.UserAtHost())
	if command != "" {
		args = append(args, command)
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// CopyToRemote copies a local file to the remote host via scp.
func CopyToRemote(host *config.RemoteHost, localPath, remotePath string) error {
	args := []string{
		"-o", "StrictHostKeyChecking=accept-new",
	}
	if host.Port != 0 {
		args = append(args, "-P", fmt.Sprintf("%d", host.Port))
	}
	if host.Identity != "" {
		args = append(args, "-i", expandHome(host.Identity))
	}

	dest := fmt.Sprintf("%s:%s", host.UserAtHost(), remotePath)
	args = append(args, localPath, dest)

	cmd := exec.Command("scp", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// TestConnection verifies SSH connectivity to the remote host.
func TestConnection(host *config.RemoteHost) error {
	args := sshArgs(host)
	args = append(args, "-o", "ConnectTimeout=5")
	args = append(args, host.UserAtHost(), "echo ok")

	cmd := exec.Command("ssh", args...)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		return fmt.Errorf("unexpected response from remote")
	}
	return nil
}

// ListRemoteDir lists entries in a directory on the remote host via SSH.
// Returns entries from `ls -1paF`, where directories have "/" suffix.
func ListRemoteDir(host *config.RemoteHost, dir string) ([]string, error) {
	// Sanitize dir to prevent command injection
	if strings.ContainsAny(dir, ";|&$`\"\\") {
		return nil, fmt.Errorf("invalid directory path")
	}
	out, err := RunSSH(host, fmt.Sprintf("ls -1paF %q 2>/dev/null", dir))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	lines := strings.Split(out, "\n")
	var entries []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "./" || line == "../" {
			continue
		}
		entries = append(entries, line)
	}
	return entries, nil
}

// expandHome expands a leading ~/ in a path to the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home + path[1:]
	}
	return path
}
