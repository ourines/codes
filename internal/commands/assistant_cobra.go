package commands

import (
	"github.com/spf13/cobra"
)

// AssistantCmd is the top-level `codes assistant` command.
var AssistantCmd = &cobra.Command{
	Use:     "assistant",
	Aliases: []string{"ai"},
	Short:   "Chat with your personal coding assistant",
	Long: `Start an interactive conversation with your AI coding assistant.
The assistant maintains conversation history per session and can create
agent teams to execute tasks in your registered projects.

Examples:
  codes assistant                          # interactive mode
  codes assistant "fix the login bug"     # one-shot
  codes assistant -s work "deploy tasks"  # named session`,
}

var assistantChatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Send a message (default subcommand)",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		model, _ := cmd.Flags().GetString("model")
		if len(args) > 0 {
			return RunAssistantOnce(joinArgs(args), session, model)
		}
		return RunAssistantREPL(session, model)
	},
}

var assistantClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear conversation history for a session",
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		return RunAssistantClear(session)
	},
}

func init() {
	// Flags shared by chat and clear
	for _, cmd := range []*cobra.Command{assistantChatCmd, assistantClearCmd} {
		cmd.Flags().StringP("session", "s", "default", "Session ID (separate histories per ID)")
	}
	assistantChatCmd.Flags().StringP("model", "m", "", "Override model (default: claude-3-5-haiku-latest)")

	AssistantCmd.AddCommand(assistantChatCmd)
	AssistantCmd.AddCommand(assistantClearCmd)

	// Make `codes assistant "message"` work without typing `chat`
	AssistantCmd.Args = cobra.ArbitraryArgs
	AssistantCmd.Flags().StringP("session", "s", "default", "Session ID")
	AssistantCmd.Flags().StringP("model", "m", "", "Override model")
	AssistantCmd.RunE = func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		model, _ := cmd.Flags().GetString("model")
		if len(args) > 0 {
			return RunAssistantOnce(joinArgs(args), session, model)
		}
		return RunAssistantREPL(session, model)
	}
}
