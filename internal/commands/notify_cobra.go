package commands

import (
	"github.com/spf13/cobra"
)

// NotifyCmd is the parent command for notification management.
var NotifyCmd = &cobra.Command{
	Use:     "notify",
	Aliases: []string{"n"},
	Short:   "Manage notification webhooks",
	Long:    "Add, remove, list, and test webhook notification endpoints",
}

// notifyAddCmd adds a webhook configuration.
var notifyAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a webhook endpoint",
	Long:  "Add a webhook notification endpoint (Slack or Feishu format)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		format, _ := cmd.Flags().GetString("format")
		events, _ := cmd.Flags().GetStringSlice("events")
		RunNotifyAdd(args[0], name, format, events)
	},
}

// notifyRemoveCmd removes a webhook configuration.
var notifyRemoveCmd = &cobra.Command{
	Use:   "remove <name-or-url>",
	Short: "Remove a webhook endpoint",
	Long:  "Remove a webhook by name or URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunNotifyRemove(args[0])
	},
}

// notifyListCmd lists all configured webhooks.
var notifyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhook endpoints",
	Long:  "List all configured webhook notification endpoints",
	Run: func(cmd *cobra.Command, args []string) {
		RunNotifyList()
	},
}

// notifyTestCmd tests a webhook by sending a test notification.
var notifyTestCmd = &cobra.Command{
	Use:   "test <name-or-url>",
	Short: "Test a webhook endpoint",
	Long:  "Send a test notification to verify webhook configuration",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunNotifyTest(args[0])
	},
}

func init() {
	// Add flags
	notifyAddCmd.Flags().StringP("name", "n", "", "Optional name for this webhook")
	notifyAddCmd.Flags().StringP("format", "f", "slack", "Webhook format: slack or feishu")
	notifyAddCmd.Flags().StringSliceP("events", "e", nil, "Event filter (task_completed, task_failed)")

	// Register subcommands
	NotifyCmd.AddCommand(notifyAddCmd)
	NotifyCmd.AddCommand(notifyRemoveCmd)
	NotifyCmd.AddCommand(notifyListCmd)
	NotifyCmd.AddCommand(notifyTestCmd)
}
