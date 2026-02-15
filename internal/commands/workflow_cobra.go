package commands

import (
	"github.com/spf13/cobra"
)

// WorkflowCmd is the parent command for workflow management.
var WorkflowCmd = &cobra.Command{
	Use:     "workflow",
	Aliases: []string{"wf"},
	Short:   "Manage and run workflows",
	Long:    "Create, list, run, and delete reusable workflow templates",
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflows",
	Run: func(cmd *cobra.Command, args []string) {
		RunWorkflowList()
	},
}

var workflowRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		model, _ := cmd.Flags().GetString("model")
		RunWorkflowRun(args[0], dir, model)
	},
}

var workflowCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new workflow template",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunWorkflowCreate(args[0])
	},
}

var workflowDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunWorkflowDelete(args[0])
	},
}

func init() {
	workflowRunCmd.Flags().StringP("dir", "d", "", "Working directory (default: current)")
	workflowRunCmd.Flags().StringP("model", "m", "", "Claude model to use")

	WorkflowCmd.AddCommand(workflowListCmd)
	WorkflowCmd.AddCommand(workflowRunCmd)
	WorkflowCmd.AddCommand(workflowCreateCmd)
	WorkflowCmd.AddCommand(workflowDeleteCmd)
}
