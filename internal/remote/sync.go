package remote

import (
	"encoding/json"
	"fmt"
	"os"

	"codes/internal/config"
)

// SyncProfiles uploads a minimal config.json (profiles + default + skipPermissions) to the remote host.
func SyncProfiles(host *config.RemoteHost) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("load local config: %w", err)
	}

	// Build a minimal remote config — only sync profile-related fields
	remoteCfg := &config.Config{
		Profiles:        cfg.Profiles,
		Default:         cfg.Default,
		SkipPermissions: cfg.SkipPermissions,
	}

	data, err := json.MarshalIndent(remoteCfg, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Write to temp file
	tmp, err := os.CreateTemp("", "codes-remote-config-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	// Ensure remote directory exists
	if _, err := RunSSH(host, "mkdir -p ~/.codes"); err != nil {
		return fmt.Errorf("create remote config dir: %w", err)
	}

	// Copy to remote
	if err := CopyToRemote(host, tmp.Name(), "~/.codes/config.json"); err != nil {
		return fmt.Errorf("copy config to remote: %w", err)
	}

	return nil
}
