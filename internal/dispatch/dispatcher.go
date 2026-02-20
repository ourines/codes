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
		// Store callback URL on every task so any completion triggers the callback
		if opts.CallbackURL != "" {
			agent.UpdateTask(teamName, task.ID, func(t *agent.Task) error {
				t.CallbackURL = opts.CallbackURL
				return nil
			})
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

// DispatchStream performs dispatch with streaming events via the onEvent callback.
// Each phase of the dispatch process emits an event so the caller can stream progress.
func DispatchStream(ctx context.Context, opts DispatchOptions, onEvent func(DispatchEvent)) {
	start := time.Now()

	if opts.UserInput == "" {
		onEvent(DispatchEvent{Event: "error", Error: "user input is required", Duration: time.Since(start).String()})
		return
	}
	if opts.Channel == "" {
		opts.Channel = "cli"
	}
	if !validChannel.MatchString(opts.Channel) {
		opts.Channel = "unknown"
	}

	// Phase: analyzing
	onEvent(DispatchEvent{Event: "status", Phase: "analyzing", Message: "Analyzing intent..."})

	apiKey, baseURL, err := resolveAPICredentials()
	if err != nil {
		onEvent(DispatchEvent{Event: "error", Error: fmt.Sprintf("resolve API credentials: %v", err), Duration: time.Since(start).String()})
		return
	}

	projectNames, err := getProjectNames()
	if err != nil {
		onEvent(DispatchEvent{Event: "error", Error: fmt.Sprintf("list projects: %v", err), Duration: time.Since(start).String()})
		return
	}

	systemPrompt, userPrompt := buildPrompt(opts.UserInput, projectNames)
	intent, err := analyzeIntent(ctx, apiKey, baseURL, opts.Model, systemPrompt, userPrompt)
	if err != nil {
		onEvent(DispatchEvent{Event: "error", Error: fmt.Sprintf("analyze intent: %v", err), Duration: time.Since(start).String()})
		return
	}

	if opts.Project != "" {
		intent.Project = opts.Project
	}

	// Clarify?
	if intent.Clarify != "" {
		onEvent(DispatchEvent{Event: "clarify", Clarify: intent.Clarify, Intent: intent, Duration: time.Since(start).String()})
		return
	}

	// Error in intent?
	if intent.Error != "" {
		onEvent(DispatchEvent{Event: "error", Error: intent.Error, Intent: intent, Duration: time.Since(start).String()})
		return
	}

	if intent.Project == "" {
		onEvent(DispatchEvent{Event: "error", Error: "no project identified from the request", Intent: intent, Duration: time.Since(start).String()})
		return
	}

	project, exists := config.GetProject(intent.Project)
	if !exists {
		onEvent(DispatchEvent{Event: "error", Error: fmt.Sprintf("project %q not found", intent.Project), Intent: intent, Duration: time.Since(start).String()})
		return
	}

	if len(intent.Tasks) == 0 {
		onEvent(DispatchEvent{Event: "error", Error: "no tasks identified from the request", Intent: intent, Duration: time.Since(start).String()})
		return
	}

	// Phase: creating team
	result, err := executeStream(intent, project, opts, onEvent)
	if err != nil {
		onEvent(DispatchEvent{Event: "error", Error: err.Error(), Duration: time.Since(start).String()})
		return
	}

	// Final result
	onEvent(DispatchEvent{
		Event:         "result",
		Phase:         "completed",
		TeamName:      result.TeamName,
		TasksCreated:  result.TasksCreated,
		AgentsStarted: result.AgentsStarted,
		Intent:        intent,
		Duration:      time.Since(start).String(),
	})
}

// executeStream is like execute but emits progress events.
func executeStream(intent *IntentResponse, project config.ProjectEntry, opts DispatchOptions, onEvent func(DispatchEvent)) (*DispatchResult, error) {
	randBuf := make([]byte, 2)
	_, _ = rand.Read(randBuf)
	randSuffix := int(randBuf[0])<<8 | int(randBuf[1])
	teamName := fmt.Sprintf("dispatch-%s-%d-%04d", opts.Channel, time.Now().UnixNano(), randSuffix%10000)

	onEvent(DispatchEvent{Event: "status", Phase: "creating_team", Message: fmt.Sprintf("Creating team %s...", teamName), TeamName: teamName})

	desc := fmt.Sprintf("Dispatch: %.200s", opts.UserInput)
	if _, err := agent.CreateTeam(teamName, desc, project.Path); err != nil {
		return nil, fmt.Errorf("create team: %w", err)
	}

	numWorkers := len(intent.Tasks)
	if numWorkers > 5 {
		numWorkers = 5
	}

	workers := workerNames(numWorkers)
	for _, name := range workers {
		if err := agent.AddMember(teamName, agent.TeamMember{
			Name: name, Role: "Execute dispatched coding tasks", Model: "sonnet", Type: "worker",
		}); err != nil {
			agent.DeleteTeam(teamName)
			return nil, fmt.Errorf("add worker %s: %w", name, err)
		}
	}

	onEvent(DispatchEvent{Event: "status", Phase: "creating_tasks", Message: fmt.Sprintf("Creating %d tasks...", len(intent.Tasks))})

	taskIDMap := make(map[int]int)
	for i, taskIntent := range intent.Tasks {
		var blockedBy []int
		for _, dep := range taskIntent.DependsOn {
			if actualID, ok := taskIDMap[dep]; ok {
				blockedBy = append(blockedBy, actualID)
			}
		}
		owner := workers[i%numWorkers]
		priority := agent.PriorityNormal
		switch taskIntent.Priority {
		case "high":
			priority = agent.PriorityHigh
		case "low":
			priority = agent.PriorityLow
		}
		task, err := agent.CreateTask(teamName, taskIntent.Subject, taskIntent.Description, owner, blockedBy, priority, intent.Project, "")
		if err != nil {
			agent.DeleteTeam(teamName)
			return nil, fmt.Errorf("create task %d: %w", i+1, err)
		}
		if opts.CallbackURL != "" {
			agent.UpdateTask(teamName, task.ID, func(t *agent.Task) error {
				t.CallbackURL = opts.CallbackURL
				return nil
			})
		}
		taskIDMap[i+1] = task.ID
	}

	onEvent(DispatchEvent{Event: "status", Phase: "starting_agents", Message: fmt.Sprintf("Starting %d agents...", numWorkers)})

	results, err := agent.StartAllAgents(teamName)
	if err != nil {
		agent.DeleteTeam(teamName)
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
