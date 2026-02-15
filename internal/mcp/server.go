package mcpserver

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunServer starts the MCP server over stdio transport.
func RunServer() error {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "codes",
			Version: "1.0.0",
		},
		nil,
	)

	// Register tools
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_projects",
		Description: "List all configured project aliases with their paths and git status",
	}, listProjectsHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add_project",
		Description: "Add a new project alias mapping a name to a directory path",
	}, addProjectHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "remove_project",
		Description: "Remove a project alias by name",
	}, removeProjectHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_profiles",
		Description: "List all API profiles with their status and settings",
	}, listProfilesHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "switch_profile",
		Description: "Switch the default API profile",
	}, switchProfileHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "get_project_info",
		Description: "Get detailed information about a project including git status and branch info",
	}, getProjectInfoHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_remotes",
		Description: "List all configured remote SSH hosts",
	}, listRemotesHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "add_remote",
		Description: "Add a new remote SSH host configuration",
	}, addRemoteHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "remove_remote",
		Description: "Remove a remote SSH host configuration by name",
	}, removeRemoteHandler)

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "sync_remote",
		Description: "Sync local API profiles and settings to a remote SSH host",
	}, syncRemoteHandler)

	// Agent team tools
	registerAgentTools(server)

	// Workflow tools
	registerWorkflowTools(server)

	// Stats tools
	registerStatsTools(server)

	return server.Run(context.Background(), &mcpsdk.StdioTransport{})
}
