package dispatch

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"time"

	"codes/internal/agent"
	"codes/internal/config"
)

var validChannel = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Dispatch analyzes user input and executes the resulting tasks.
// It performs intent analysis via Claude API, then creates a team with workers and tasks.
func Dispatch(ctx context.Context, opts DispatchOptions) (*DispatchResult, error) {
	start := time.Now()

	if opts.UserInput == "" {
		return nil, fmt.Errorf("user input is required")
	}
	if opts.Channel == "" {
		opts.Channel = "cli"
	}
	if !validChannel.MatchString(opts.Channel) {
		opts.Channel = "unknown"
	}

	// Resolve API credentials from active profile
	apiKey, baseURL, err := resolveAPICredentials()
	if err != nil {
		return nil, fmt.Errorf("resolve API credentials: %w", err)
	}

	// Get available project names for the prompt
	projectNames, err := getProjectNames()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	// Build prompt and call API for intent analysis
	systemPrompt, userPrompt := buildPrompt(opts.UserInput, projectNames)
	intent, err := analyzeIntent(ctx, apiKey, baseURL, opts.Model, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("analyze intent: %w", err)
	}

	// If user specified a project, override the intent's project
	if opts.Project != "" {
		intent.Project = opts.Project
	}

	// If clarification needed, return early
	if intent.Clarify != "" {
		return &DispatchResult{
			Clarify:     intent.Clarify,
			Intent:      intent,
			DurationStr: time.Since(start).String(),
		}, nil
	}

	// If error in intent, return early
	if intent.Error != "" {
		return &DispatchResult{
			Error:       intent.Error,
			Intent:      intent,
			DurationStr: time.Since(start).String(),
		}, nil
	}

	// Validate project exists
	if intent.Project == "" {
		return &DispatchResult{
			Error:       "no project identified from the request",
			Intent:      intent,
			DurationStr: time.Since(start).String(),
		}, nil
	}

	project, exists := config.GetProject(intent.Project)
	if !exists {
		return &DispatchResult{
			Error:       fmt.Sprintf("project %q not found in configuration", intent.Project),
			Intent:      intent,
			DurationStr: time.Since(start).String(),
		}, nil
	}

	if len(intent.Tasks) == 0 {
		return &DispatchResult{
			Error:       "no tasks were identified from the request",
			Intent:      intent,
			DurationStr: time.Since(start).String(),
		}, nil
	}

	// Execute: create team, add workers, create tasks, start agents
	result, err := execute(intent, project, opts)
	if err != nil {
		return nil, err
	}
	result.Intent = intent
	result.DurationStr = time.Since(start).String()
	return result, nil
}

// execute creates the team, workers, tasks and starts agents.
func execute(intent *IntentResponse, project config.ProjectEntry, opts DispatchOptions) (*DispatchResult, error) {
	// Use nanosecond timestamp + 4-digit random suffix to avoid collisions
	randBuf := make([]byte, 2)
	_, _ = rand.Read(randBuf)
	randSuffix := int(randBuf[0])<<8 | int(randBuf[1])
	teamName := fmt.Sprintf("dispatch-%s-%d-%04d", opts.Channel, time.Now().UnixNano(), randSuffix%10000)

	// Truncate user input in team description to prevent oversized metadata
	desc := fmt.Sprintf("Dispatch: %.200s", opts.UserInput)

	// Create team
	if _, err := agent.CreateTeam(teamName, desc, project.Path); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	// Determine number of workers (1 per task, max 5)
	numWorkers := len(intent.Tasks)
	if numWorkers > 5 {
		numWorkers = 5
	}

	// Add worker agents
	workers := workerNames(numWorkers)
	for _, name := range workers {
		if err := agent.AddMember(teamName, agent.TeamMember{
			Name:  name,
			Role:  "Execute dispatched coding tasks",
			Model: "sonnet",
			Type:  "worker",
		}); err != nil {
			// Rollback: delete the team on failure
			cleanupErr := agent.DeleteTeam(teamName)
			if cleanupErr != nil {
				return nil, fmt.Errorf("add worker %s: %w (cleanup also failed: %v)", name, err, cleanupErr)
			}
			return nil, fmt.Errorf("add worker %s: %w", name, err)
		}
	}

	// Create tasks with dependency mapping
	// Intent uses 1-based dependsOn indices; we need to map to actual task IDs
	taskIDMap := make(map[int]int) // 1-based intent index -> actual task ID
	for i, taskIntent := range intent.Tasks {
		// Map dependsOn from 1-based intent indices to actual task IDs
		var blockedBy []int
		for _, dep := range taskIntent.DependsOn {
			if actualID, ok := taskIDMap[dep]; ok {
				blockedBy = append(blockedBy, actualID)
			}
		}

		// Assign to workers round-robin
		owner := workers[i%numWorkers]

		priority := agent.PriorityNormal
		switch taskIntent.Priority {
		case "high":
			priority = agent.PriorityHigh
		case "low":
			priority = agent.PriorityLow
		}

		task, err := agent.CreateTask(
			teamName,
			taskIntent.Subject,
			taskIntent.Description,
			owner,
			blockedBy,
			priority,
			intent.Project,
			"",
		)
		if err != nil {
			cleanupErr := agent.DeleteTeam(teamName)
			if cleanupErr != nil {
				return nil, fmt.Errorf("create task %d: %w (cleanup also failed: %v)", i+1, err, cleanupErr)
			}
			return nil, fmt.Errorf("create task %d: %w", i+1, err)
		}
		taskIDMap[i+1] = task.ID
	}

	// Start all agents
	results, err := agent.StartAllAgents(teamName)
	if err != nil {
		cleanupErr := agent.DeleteTeam(teamName)
		if cleanupErr != nil {
			return nil, fmt.Errorf("start agents: %w (cleanup also failed: %v)", err, cleanupErr)
		}
		return nil, fmt.Errorf("start agents: %w", err)
	}

	agentsStarted := 0
	for _, r := range results {
		if r.Started {
			agentsStarted++
		}
	}

	return &DispatchResult{
		TeamName:      teamName,
		TasksCreated:  len(intent.Tasks),
		AgentsStarted: agentsStarted,
	}, nil
}

// resolveAPICredentials loads the API key and base URL from the active profile.
func resolveAPICredentials() (apiKey, baseURL string, err error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return "", "", err
	}

	// Find the active profile
	var activeProfile *config.APIConfig
	for i := range cfg.Profiles {
		if cfg.Profiles[i].Name == cfg.Default {
			activeProfile = &cfg.Profiles[i]
			break
		}
	}
	if activeProfile == nil && len(cfg.Profiles) > 0 {
		activeProfile = &cfg.Profiles[0]
	}
	if activeProfile == nil {
		return "", "", fmt.Errorf("no API profiles configured")
	}

	envVars := config.GetEnvironmentVars(activeProfile)
	apiKey = envVars["ANTHROPIC_AUTH_TOKEN"]
	if apiKey == "" {
		apiKey = envVars["ANTHROPIC_API_KEY"]
	}
	baseURL = envVars["ANTHROPIC_BASE_URL"]

	if apiKey == "" {
		return "", "", fmt.Errorf("no API key found in active profile %q", activeProfile.Name)
	}

	return apiKey, baseURL, nil
}

// getProjectNames returns the names of all registered projects.
func getProjectNames() ([]string, error) {
	projects, err := config.ListProjects()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	return names, nil
}
