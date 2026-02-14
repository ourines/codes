package commands

import (
	"github.com/spf13/cobra"
)

// AgentCmd is the parent command for agent/team management.
var AgentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"a"},
	Short:   "Manage agent teams and tasks",
	Long:    "Create teams of Claude agents, assign tasks, and coordinate work through message passing",
}

// -- Team subcommands --

var agentTeamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage teams",
	Long:  "Create, delete, and list agent teams",
}

var agentTeamCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new team",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		desc, _ := cmd.Flags().GetString("description")
		workdir, _ := cmd.Flags().GetString("workdir")
		RunAgentTeamCreate(args[0], desc, workdir)
	},
}

var agentTeamDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a team and all its data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentTeamDelete(args[0])
	},
}

var agentTeamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all teams",
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentTeamList()
	},
}

var agentTeamInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show team details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentTeamInfo(args[0])
	},
}

// -- Agent member subcommands --

var agentAddCmd = &cobra.Command{
	Use:   "add <team> <name>",
	Short: "Add an agent to a team",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		role, _ := cmd.Flags().GetString("role")
		model, _ := cmd.Flags().GetString("model")
		agentType, _ := cmd.Flags().GetString("type")
		RunAgentAdd(args[0], args[1], role, model, agentType)
	},
}

var agentRemoveCmd = &cobra.Command{
	Use:   "remove <team> <name>",
	Short: "Remove an agent from a team",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentRemove(args[0], args[1])
	},
}

var agentStartCmd = &cobra.Command{
	Use:   "start <team> <name>",
	Short: "Start an agent daemon",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentStart(args[0], args[1])
	},
}

var agentStopCmd = &cobra.Command{
	Use:   "stop <team> <name>",
	Short: "Stop an agent daemon",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentStop(args[0], args[1])
	},
}

var agentRunCmd = &cobra.Command{
	Use:    "run <team> <name>",
	Short:  "Run agent daemon (internal)",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentDaemon(args[0], args[1])
	},
}

// -- Task subcommands --

var agentTaskCmd = &cobra.Command{
	Use:   "task",
	Short: "Manage tasks",
	Long:  "Create, list, and manage tasks for a team",
}

var agentTaskCreateCmd = &cobra.Command{
	Use:   "create <team> <subject>",
	Short: "Create a new task",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		desc, _ := cmd.Flags().GetString("description")
		assign, _ := cmd.Flags().GetString("assign")
		blockedBy, _ := cmd.Flags().GetIntSlice("blocked-by")
		priority, _ := cmd.Flags().GetString("priority")
		project, _ := cmd.Flags().GetString("project")
		workDir, _ := cmd.Flags().GetString("work-dir")
		RunAgentTaskCreate(args[0], args[1], desc, assign, blockedBy, priority, project, workDir)
	},
}

var agentTaskListCmd = &cobra.Command{
	Use:   "list <team>",
	Short: "List tasks",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")
		owner, _ := cmd.Flags().GetString("owner")
		RunAgentTaskList(args[0], status, owner)
	},
}

var agentTaskGetCmd = &cobra.Command{
	Use:   "get <team> <task-id>",
	Short: "Get task details",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentTaskGet(args[0], args[1])
	},
}

var agentTaskCancelCmd = &cobra.Command{
	Use:   "cancel <team> <task-id>",
	Short: "Cancel a task",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentTaskCancel(args[0], args[1])
	},
}

// -- Message subcommands --

var agentMessageCmd = &cobra.Command{
	Use:   "message",
	Short: "Manage messages",
	Long:  "Send and list messages between agents",
}

var agentMessageSendCmd = &cobra.Command{
	Use:   "send <team> <content>",
	Short: "Send a message",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		from, _ := cmd.Flags().GetString("from")
		to, _ := cmd.Flags().GetString("to")
		RunAgentMessageSend(args[0], from, to, args[1])
	},
}

var agentMessageListCmd = &cobra.Command{
	Use:   "list <team>",
	Short: "List messages for an agent",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		agentName, _ := cmd.Flags().GetString("agent")
		RunAgentMessageList(args[0], agentName)
	},
}

// -- Status command --

var agentStatusCmd = &cobra.Command{
	Use:   "status <team>",
	Short: "Show team dashboard",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			RunAgentStatusWatch(args[0])
		} else {
			RunAgentStatus(args[0])
		}
	},
}

// -- Start-all / Stop-all commands --

var agentStartAllCmd = &cobra.Command{
	Use:   "start-all <team>",
	Short: "Start all agent daemons in a team",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentStartAll(args[0])
	},
}

var agentStopAllCmd = &cobra.Command{
	Use:   "stop-all <team>",
	Short: "Stop all agent daemons in a team",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunAgentStopAll(args[0])
	},
}

func init() {
	// Team commands
	agentTeamCreateCmd.Flags().String("description", "", "Team description")
	agentTeamCreateCmd.Flags().String("workdir", "", "Working directory for agents")
	agentTeamCmd.AddCommand(agentTeamCreateCmd, agentTeamDeleteCmd, agentTeamListCmd, agentTeamInfoCmd)

	// Agent member commands
	agentAddCmd.Flags().String("role", "", "Agent role description")
	agentAddCmd.Flags().String("model", "", "Claude model to use (e.g. sonnet, opus)")
	agentAddCmd.Flags().String("type", "worker", "Agent type (worker, leader)")

	// Task commands
	agentTaskCreateCmd.Flags().StringP("description", "d", "", "Task description")
	agentTaskCreateCmd.Flags().String("assign", "", "Assign to agent")
	agentTaskCreateCmd.Flags().IntSlice("blocked-by", nil, "Task IDs that block this task")
	agentTaskCreateCmd.Flags().String("priority", "normal", "Task priority: high, normal, or low")
	agentTaskCreateCmd.Flags().StringP("project", "p", "", "Project name to execute in (registered via codes project add)")
	agentTaskCreateCmd.Flags().String("work-dir", "", "Explicit working directory (overrides project)")
	agentTaskListCmd.Flags().String("status", "", "Filter by status")
	agentTaskListCmd.Flags().String("owner", "", "Filter by owner")
	agentTaskCmd.AddCommand(agentTaskCreateCmd, agentTaskListCmd, agentTaskGetCmd, agentTaskCancelCmd)

	// Message commands
	agentMessageSendCmd.Flags().String("from", "", "Sender agent name")
	agentMessageSendCmd.Flags().String("to", "", "Recipient agent name (empty for broadcast)")
	agentMessageSendCmd.MarkFlagRequired("from")
	agentMessageListCmd.Flags().String("agent", "", "Agent name to list messages for")
	agentMessageListCmd.MarkFlagRequired("agent")
	agentMessageCmd.AddCommand(agentMessageSendCmd, agentMessageListCmd)

	// Status flags
	agentStatusCmd.Flags().BoolP("watch", "w", false, "Auto-refresh every 3 seconds")

	// Build command tree
	AgentCmd.AddCommand(agentTeamCmd)
	AgentCmd.AddCommand(agentAddCmd)
	AgentCmd.AddCommand(agentRemoveCmd)
	AgentCmd.AddCommand(agentStartCmd)
	AgentCmd.AddCommand(agentStopCmd)
	AgentCmd.AddCommand(agentStartAllCmd)
	AgentCmd.AddCommand(agentStopAllCmd)
	AgentCmd.AddCommand(agentRunCmd)
	AgentCmd.AddCommand(agentTaskCmd)
	AgentCmd.AddCommand(agentMessageCmd)
	AgentCmd.AddCommand(agentStatusCmd)
}
