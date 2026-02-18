package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// platformArchiveName returns the expected release archive name for the current platform.
func platformArchiveName(tag string) string {
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("codes-%s-%s-%s%s", tag, runtime.GOOS, runtime.GOARCH, ext)
}

// platformBinaryName returns the binary name inside the archive.
func platformBinaryName() string {
	if runtime.GOOS == "windows" {
		return "codes.exe"
	}
	return "codes"
}

// downloadURL returns the GitHub release download URL for the given tag.
func downloadURL(tag string) string {
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/%s/%s",
		repoOwner, repoName, tag, platformArchiveName(tag),
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

// DownloadRelease downloads and extracts the binary for the given release.
// Returns the path of the extracted binary.
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

	// Save archive to temp file
	archivePath := filepath.Join(destDir, platformArchiveName(release.TagName))
	f, err := os.Create(archivePath)
	if err != nil {
		return "", fmt.Errorf("create archive file: %w", err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(archivePath)
		return "", fmt.Errorf("write failed: %w", err)
	}
	f.Close()

	// Extract binary from archive
	binaryName := platformBinaryName()
	destPath := filepath.Join(destDir, binaryName)

	if runtime.GOOS == "windows" {
		err = extractFromZip(archivePath, binaryName, destPath)
	} else {
		err = extractFromTarGz(archivePath, binaryName, destPath)
	}
	os.Remove(archivePath)
	if err != nil {
		return "", fmt.Errorf("extract failed: %w", err)
	}

	if err := os.Chmod(destPath, 0755); err != nil {
		os.Remove(destPath)
		return "", err
	}

	return destPath, nil
}

// extractFromTarGz extracts a single file from a .tar.gz archive.
func extractFromTarGz(archivePath, targetName, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(hdr.Name) == targetName && hdr.Typeflag == tar.TypeReg {
			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, tr)
			return err
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
}

// extractFromZip extracts a single file from a .zip archive.
func extractFromZip(archivePath, targetName, destPath string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, zf := range r.File {
		if filepath.Base(zf.Name) == targetName {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer out.Close()
			_, err = io.Copy(out, rc)
			return err
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
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
	codesignDarwin(self)
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
	release, err := CheckLatestVersion()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if currentVer != "dev" && !CompareVersions(currentVer, release.TagName) {
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

// codesignDarwin re-signs a binary with an ad-hoc signature on macOS.
// This prevents SIGKILL from macOS AMFI (Apple Mobile File Integrity) which
// can reject binaries whose code signing cache is stale or was created on a
// different machine (e.g. CI runners).
func codesignDarwin(path string) {
	if runtime.GOOS != "darwin" {
		return
	}
	cmd := exec.Command("codesign", "--force", "--sign", "-", path)
	_ = cmd.Run() // best-effort; ignore errors
}
