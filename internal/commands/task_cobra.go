package commands

import (
	"github.com/spf13/cobra"
)

// TaskSimpleCmd is the top-level shorthand for task management.
var TaskSimpleCmd = &cobra.Command{
	Use:     "task",
	Aliases: []string{"t"},
	Short:   "Quick task management",
	Long:    "Simplified task management — add, list, and view task results",
}

var taskSimpleAddCmd = &cobra.Command{
	Use:   "add <team> <description>",
	Short: "Add a task to a team",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		assign, _ := cmd.Flags().GetString("assign")
		RunTaskSimpleAdd(args[0], args[1], assign)
	},
}

var taskSimpleListCmd = &cobra.Command{
	Use:   "list [team]",
	Short: "List tasks",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		team := ""
		if len(args) > 0 {
			team = args[0]
		}
		RunTaskSimpleList(team)
	},
}

var taskSimpleResultCmd = &cobra.Command{
	Use:   "result <team> <task-id>",
	Short: "Show task result",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunTaskSimpleResult(args[0], args[1])
	},
}

func init() {
	taskSimpleAddCmd.Flags().StringP("assign", "a", "", "Assign to a specific agent")

	TaskSimpleCmd.AddCommand(taskSimpleAddCmd)
	TaskSimpleCmd.AddCommand(taskSimpleListCmd)
	TaskSimpleCmd.AddCommand(taskSimpleResultCmd)
}
