package update

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// platformBinaryName returns the expected binary name for the current platform.
func platformBinaryName() string {
	name := fmt.Sprintf("codes-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// downloadURL returns the GitHub release download URL for the given tag.
func downloadURL(tag string) string {
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		repoOwner, repoName, tag, platformBinaryName(),
	)
}

// stagingDirPath returns ~/.codes/update/.
func stagingDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codes", "update"), nil
}

// DownloadRelease downloads the binary for the given release to destDir.
// Returns the path of the downloaded file.
func DownloadRelease(release *ReleaseInfo, destDir string) (string, error) {
	url := downloadURL(release.TagName)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	destPath := filepath.Join(destDir, platformBinaryName())
	f, err := os.CreateTemp(destDir, "codes-update-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write failed: %w", err)
	}
	f.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return destPath, nil
}

// ReplaceSelf replaces the currently running binary with newBinaryPath.
func ReplaceSelf(newBinaryPath string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("cannot resolve symlinks: %w", err)
	}

	if runtime.GOOS == "windows" {
		// Windows: can't overwrite running binary; rename current to .old first
		oldPath := self + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(self, oldPath); err != nil {
			return fmt.Errorf("cannot rename current binary: %w (try running as admin)", err)
		}
		if err := copyFile(newBinaryPath, self); err != nil {
			// Attempt rollback
			_ = os.Rename(oldPath, self)
			return fmt.Errorf("cannot write new binary: %w", err)
		}
		_ = os.Remove(oldPath)
	} else {
		// Unix: atomic rename
		if err := os.Rename(newBinaryPath, self); err != nil {
			// Fallback: copy if rename fails (cross-device)
			if err := copyFile(newBinaryPath, self); err != nil {
				return fmt.Errorf("cannot replace binary: %w", err)
			}
			os.Remove(newBinaryPath)
		}
	}
	return nil
}

// ApplyStaged checks for a staged binary in ~/.codes/update/ and applies it.
func ApplyStaged() error {
	stagingDir, err := stagingDirPath()
	if err != nil {
		return err
	}
	staged := filepath.Join(stagingDir, platformBinaryName())
	if _, err := os.Stat(staged); os.IsNotExist(err) {
		return nil // nothing staged
	}

	err = ReplaceSelf(staged)
	if err != nil {
		return err
	}

	// Clean up staging directory
	_ = os.RemoveAll(stagingDir)
	return nil
}

// RunSelfUpdate performs a manual self-update: check → download → replace.
func RunSelfUpdate(currentVer string) error {
	if currentVer == "dev" {
		return fmt.Errorf("development build, skipping update check")
	}

	release, err := CheckLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !CompareVersions(currentVer, release.TagName) {
		fmt.Printf("Already up to date (%s)\n", currentVer)
		return nil
	}

	fmt.Printf("Updating %s → %s ...\n", currentVer, release.TagName)

	tmpDir, err := os.MkdirTemp("", "codes-update-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	path, err := DownloadRelease(release, tmpDir)
	if err != nil {
		return err
	}

	if err := ReplaceSelf(path); err != nil {
		return err
	}

	fmt.Printf("Successfully updated to %s\n", release.TagName)

	// Clear any stale state
	state := UpdateState{
		LastCheck:     time.Now().Unix(),
		LatestVersion: release.TagName,
	}
	saveState(state)

	return nil
}

// copyFile copies src to dst, preserving permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
