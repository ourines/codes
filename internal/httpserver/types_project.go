package httpserver

// ProjectListResponse represents the list of projects.
type ProjectListResponse struct {
	Projects []ProjectInfoResponse `json:"projects"`
}

// ProjectInfoResponse represents a project entry in API responses.
type ProjectInfoResponse struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Host string `json:"host,omitempty"`
}

// ProfileListResponse represents the list of API profiles.
type ProfileListResponse struct {
	Profiles []ProfileInfo `json:"profiles"`
}

// ProfileInfo represents a safe (no secrets) view of an API profile.
type ProfileInfo struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
}

// SwitchProfileRequest represents a request to switch the active profile.
type SwitchProfileRequest struct {
	Name string `json:"name"`
}

// SwitchProfileResponse represents the result of switching profiles.
type SwitchProfileResponse struct {
	Message string `json:"message"`
	Active  string `json:"active"`
}
