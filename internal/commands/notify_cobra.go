package commands

import (
	"github.com/spf13/cobra"
)

// NotifyCmd is the parent command for notification management.
var NotifyCmd = &cobra.Command{
	Use:     "notify",
	Aliases: []string{"n"},
	Short:   "Manage notification webhooks and hooks",
	Long:    "Add, remove, list, and test webhook notification endpoints and shell hooks",
}

// notifyAddCmd adds a webhook configuration.
var notifyAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Add a webhook endpoint",
	Long: `Add a webhook notification endpoint.

Supported formats:
  slack      Slack incoming webhook (default)
  feishu     Feishu/Lark bot webhook
  dingtalk   DingTalk bot webhook
  telegram   Telegram Bot API (requires --extra chat_id=<id>)
  custom     Custom JSON template (requires --extra template="<json>")

Examples:
  codes notify add https://hooks.slack.com/xxx
  codes notify add https://oapi.dingtalk.com/robot/send?token=xxx -f dingtalk
  codes notify add https://api.telegram.org/bot<token>/sendMessage -f telegram --extra chat_id=123456
  codes notify add https://example.com/webhook -f custom --extra 'template={"text":"{{.Text}}"}'`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		format, _ := cmd.Flags().GetString("format")
		events, _ := cmd.Flags().GetStringSlice("events")
		extra, _ := cmd.Flags().GetStringToString("extra")
		RunNotifyAdd(args[0], name, format, events, extra)
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

// hookCmd is the parent command for shell hook management.
var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Manage shell hooks",
	Long: `Manage shell hook scripts that execute on task events.

Available events:
  on_task_completed   Triggered when an agent task completes successfully
  on_task_failed      Triggered when an agent task fails

Hook scripts receive a JSON payload via stdin with task details.`,
}

// hookSetCmd sets a hook for an event.
var hookSetCmd = &cobra.Command{
	Use:   "set <event> <script-path>",
	Short: "Set a hook script for an event",
	Long: `Set a shell script to execute when the specified event occurs.

Valid events: on_task_completed, on_task_failed

The script must exist and be executable. It will receive a JSON payload
via stdin containing: team, taskId, subject, status, agent, result/error, timestamp.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		RunHookSet(args[0], args[1])
	},
}

// hookRemoveCmd removes a hook for an event.
var hookRemoveCmd = &cobra.Command{
	Use:   "remove <event>",
	Short: "Remove a hook for an event",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunHookRemove(args[0])
	},
}

// hookListCmd lists all configured hooks.
var hookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured hooks",
	Run: func(cmd *cobra.Command, args []string) {
		RunHookList()
	},
}

// hookTestCmd tests a hook by sending a mock payload.
var hookTestCmd = &cobra.Command{
	Use:   "test <event>",
	Short: "Test a hook with a mock payload",
	Long:  "Execute the hook script for the given event with a simulated task payload",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		RunHookTest(args[0])
	},
}

func init() {
	// Add flags
	notifyAddCmd.Flags().StringP("name", "n", "", "Optional name for this webhook")
	notifyAddCmd.Flags().StringP("format", "f", "slack", "Webhook format: slack, feishu, dingtalk, telegram, custom")
	notifyAddCmd.Flags().StringSliceP("events", "e", nil, "Event filter (task_completed, task_failed)")
	notifyAddCmd.Flags().StringToStringP("extra", "x", nil, "Format-specific parameters (e.g., chat_id=123456)")

	// Register webhook subcommands
	NotifyCmd.AddCommand(notifyAddCmd)
	NotifyCmd.AddCommand(notifyRemoveCmd)
	NotifyCmd.AddCommand(notifyListCmd)
	NotifyCmd.AddCommand(notifyTestCmd)

	// Register hook subcommands
	hookCmd.AddCommand(hookSetCmd)
	hookCmd.AddCommand(hookRemoveCmd)
	hookCmd.AddCommand(hookListCmd)
	hookCmd.AddCommand(hookTestCmd)
	NotifyCmd.AddCommand(hookCmd)
}
