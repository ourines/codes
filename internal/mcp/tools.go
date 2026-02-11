package mcpserver

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"codes/internal/config"
)

// list_projects

type listProjectsInput struct{}

type listProjectsOutput struct {
	Projects []config.ProjectInfo `json:"projects"`
}

func listProjectsHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input listProjectsInput) (*mcpsdk.CallToolResult, listProjectsOutput, error) {
	projects, err := config.ListProjects()
	if err != nil {
		return nil, listProjectsOutput{}, fmt.Errorf("failed to list projects: %w", err)
	}

	infos := make([]config.ProjectInfo, 0, len(projects))
	for name, path := range projects {
		infos = append(infos, config.GetProjectInfo(name, path))
	}

	return nil, listProjectsOutput{Projects: infos}, nil
}

// add_project

type addProjectInput struct {
	Name string `json:"name" jsonschema:"Project alias name"`
	Path string `json:"path" jsonschema:"Absolute path to project directory"`
}

type addProjectOutput struct {
	Added bool `json:"added"`
}

func addProjectHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input addProjectInput) (*mcpsdk.CallToolResult, addProjectOutput, error) {
	if input.Name == "" || input.Path == "" {
		return nil, addProjectOutput{}, fmt.Errorf("name and path are required")
	}
	if err := config.AddProject(input.Name, input.Path); err != nil {
		return nil, addProjectOutput{}, fmt.Errorf("failed to add project: %w", err)
	}
	return nil, addProjectOutput{Added: true}, nil
}

// remove_project

type removeProjectInput struct {
	Name string `json:"name" jsonschema:"Project alias name to remove"`
}

type removeProjectOutput struct {
	Removed bool `json:"removed"`
}

func removeProjectHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input removeProjectInput) (*mcpsdk.CallToolResult, removeProjectOutput, error) {
	if err := config.RemoveProject(input.Name); err != nil {
		return nil, removeProjectOutput{}, fmt.Errorf("failed to remove project: %w", err)
	}
	return nil, removeProjectOutput{Removed: true}, nil
}

// list_profiles

type listProfilesInput struct{}

type profileInfo struct {
	Name            string `json:"name"`
	Status          string `json:"status"`
	SkipPermissions bool   `json:"skipPermissions"`
	EnvCount        int    `json:"envCount"`
}

type listProfilesOutput struct {
	Profiles []profileInfo `json:"profiles"`
	Default  string        `json:"default"`
}

func listProfilesHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input listProfilesInput) (*mcpsdk.CallToolResult, listProfilesOutput, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, listProfilesOutput{}, fmt.Errorf("failed to load config: %w", err)
	}

	infos := make([]profileInfo, 0, len(cfg.Profiles))
	for _, c := range cfg.Profiles {
		skip := false
		if c.SkipPermissions != nil {
			skip = *c.SkipPermissions
		}
		infos = append(infos, profileInfo{
			Name:            c.Name,
			Status:          c.Status,
			SkipPermissions: skip,
			EnvCount:        len(c.Env),
		})
	}

	return nil, listProfilesOutput{Profiles: infos, Default: cfg.Default}, nil
}

// switch_profile

type switchProfileInput struct {
	Name string `json:"name" jsonschema:"Profile name to set as default"`
}

type switchProfileOutput struct {
	Switched bool   `json:"switched"`
	Default  string `json:"default"`
}

func switchProfileHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input switchProfileInput) (*mcpsdk.CallToolResult, switchProfileOutput, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, switchProfileOutput{}, fmt.Errorf("failed to load config: %w", err)
	}

	found := false
	for _, c := range cfg.Profiles {
		if c.Name == input.Name {
			found = true
			break
		}
	}
	if !found {
		return nil, switchProfileOutput{}, fmt.Errorf("profile %q not found", input.Name)
	}

	cfg.Default = input.Name
	if err := config.SaveConfig(cfg); err != nil {
		return nil, switchProfileOutput{}, fmt.Errorf("failed to save config: %w", err)
	}

	return nil, switchProfileOutput{Switched: true, Default: input.Name}, nil
}

// get_project_info

type getProjectInfoInput struct {
	Name string `json:"name" jsonschema:"Project alias name"`
}

type getProjectInfoOutput struct {
	config.ProjectInfo
}

func getProjectInfoHandler(ctx context.Context, req *mcpsdk.CallToolRequest, input getProjectInfoInput) (*mcpsdk.CallToolResult, getProjectInfoOutput, error) {
	path, exists := config.GetProjectPath(input.Name)
	if !exists {
		return nil, getProjectInfoOutput{}, fmt.Errorf("project %q not found", input.Name)
	}

	info := config.GetProjectInfo(input.Name, path)
	return nil, getProjectInfoOutput{ProjectInfo: info}, nil
}
