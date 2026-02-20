package commands

import (
	"github.com/spf13/cobra"
)

// DispatchCmd represents the dispatch command
var DispatchCmd = &cobra.Command{
	Use:   "dispatch <text>",
	Short: "Dispatch a task using AI intent analysis",
	Long: `Analyze a natural language request with AI, identify the target project, break it into tasks, and start an agent team to execute them.`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		model, _ := cmd.Flags().GetString("model")
		callbackURL, _ := cmd.Flags().GetString("callback-url")
		RunDispatch(joinArgs(args), project, model, callbackURL)
	},
}

func init() {
	DispatchCmd.Flags().StringP("project", "p", "", "Target project name (auto-detected if not set)")
	DispatchCmd.Flags().StringP("model", "m", "", "Model for intent analysis (default: haiku)")
	DispatchCmd.Flags().String("callback-url", "", "URL to call when tasks complete")
}
