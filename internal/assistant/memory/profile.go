package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Profile holds structured information about the user.
// It is injected into the system prompt on every assistant turn.
type Profile struct {
	Name           string `json:"name"`
	Timezone       string `json:"timezone"`
	Language       string `json:"language"`       // "zh" or "en"
	DefaultProject string `json:"default_project"`
	Notes          string `json:"notes"` // free-form personal notes
}

// profilePath returns the full path to profile.json.
func profilePath() (string, error) {
	dir, err := memoryDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "profile.json"), nil
}

// LoadProfile loads the user profile from disk.
// Returns an empty (zero-value) Profile if the file does not exist.
func LoadProfile() (*Profile, error) {
	path, err := profilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Profile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read profile: %w", err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		// Corrupted profile — return empty rather than failing hard.
		return &Profile{}, nil
	}
	return &p, nil
}

// SaveProfile persists the profile to disk atomically.
func SaveProfile(p *Profile) error {
	path, err := profilePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal profile: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write profile tmp: %w", err)
	}
	return os.Rename(tmp, path)
}

// UpdateProfile updates a single field by name.
// Accepted field names: name, timezone, language, default_project, notes.
func UpdateProfile(field, value string) error {
	p, err := LoadProfile()
	if err != nil {
		return err
	}

	switch field {
	case "name":
		p.Name = value
	case "timezone":
		p.Timezone = value
	case "language":
		p.Language = value
	case "default_project":
		p.DefaultProject = value
	case "notes":
		p.Notes = value
	default:
		return fmt.Errorf("unknown profile field %q; valid fields: name, timezone, language, default_project, notes", field)
	}

	return SaveProfile(p)
}
