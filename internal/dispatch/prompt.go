package dispatch

import (
	"fmt"
	"strings"
)

// buildPrompt constructs the system and user prompts for intent analysis.
func buildPrompt(userInput string, projectNames []string) (system, user string) {
	projectList := "None registered"
	if len(projectNames) > 0 {
		projectList = strings.Join(projectNames, ", ")
	}

	system = fmt.Sprintf(`You are a task dispatcher for a coding agent system called "codes".
Your job is to analyze user requests and convert them into structured task plans.

Available projects: %s

Rules:
1. Match the user's request to one of the available projects. If the user mentions a project name, use it directly.
2. Break complex requests into atomic tasks that can be executed independently or with dependencies.
3. Each task should have a clear subject and description that a coding agent can act on.
4. If the user's intent is unclear or ambiguous, set the "clarify" field with a question.
5. If the request is not a coding/development task, set the "error" field.
6. Priority defaults to "normal". Use "high" for urgent/blocking issues, "low" for nice-to-haves.
7. Use dependsOn (1-based task indices) when tasks must execute in order.

Respond with ONLY a JSON object in this exact format:
{
  "project": "project-name",
  "tasks": [
    {
      "subject": "Brief task title",
      "description": "Detailed description of what the agent should do",
      "priority": "normal",
      "dependsOn": []
    }
  ],
  "clarify": "",
  "error": ""
}`, projectList)

	user = userInput
	return system, user
}

// workerName generates a worker name for the given index.
func workerName(index int) string {
	return fmt.Sprintf("worker-%d", index+1)
}

// workerNames generates worker names for the given count.
func workerNames(count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = workerName(i)
	}
	return names
}
