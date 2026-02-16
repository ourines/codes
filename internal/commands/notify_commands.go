package commands

import (
	"codes/internal/config"
	"codes/internal/notify"
	"codes/internal/output"
	"codes/internal/ui"
	"fmt"
	"strings"
	"time"
)

// RunNotifyAdd adds a webhook configuration.
func RunNotifyAdd(url, name, format string, events []string, extra map[string]string) {
	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		ui.ShowError("Invalid webhook URL: must start with http:// or https://", nil)
		return
	}

	// Validate format
	validFormats := map[string]bool{"slack": true, "feishu": true, "dingtalk": true, "telegram": true, "custom": true}
	if !validFormats[format] {
		ui.ShowError("Invalid format: must be one of 'slack', 'feishu', 'dingtalk', 'telegram', 'custom'", nil)
		return
	}

	// Validate format-specific requirements
	if format == "telegram" {
		if extra == nil || extra["chat_id"] == "" {
			ui.ShowError("Telegram format requires --extra chat_id=<id>", nil)
			return
		}
	}
	if format == "custom" {
		if extra == nil || extra["template"] == "" {
			ui.ShowError("Custom format requires --extra template=\"<json>\"", nil)
			return
		}
	}

	webhook := config.WebhookConfig{
		Name:   name,
		URL:    url,
		Format: format,
		Events: events,
		Extra:  extra,
	}

	if err := config.AddWebhook(webhook); err != nil {
		ui.ShowError("Failed to add webhook", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"webhook": webhook,
		})
		return
	}

	ui.ShowSuccess("Webhook added successfully")
	if name != "" {
		fmt.Printf("  Name: %s\n", name)
	}
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Format: %s\n", format)
	if len(events) > 0 {
		fmt.Printf("  Events: %s\n", strings.Join(events, ", "))
	}
	if len(extra) > 0 {
		fmt.Printf("  Extra: %s\n", formatExtra(extra))
	}
}

// RunNotifyRemove removes a webhook configuration.
func RunNotifyRemove(identifier string) {
	if err := config.RemoveWebhook(identifier); err != nil {
		ui.ShowError("Failed to remove webhook", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"removed": identifier,
		})
		return
	}

	ui.ShowSuccess("Webhook removed: %s", identifier)
}

// RunNotifyList lists all configured webhooks.
func RunNotifyList() {
	webhooks, err := config.ListWebhooks()
	if err != nil {
		ui.ShowError("Failed to list webhooks", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"webhooks": webhooks,
			"count":    len(webhooks),
		})
		return
	}

	if len(webhooks) == 0 {
		fmt.Println("No webhooks configured")
		fmt.Println("\nAdd a webhook with:")
		fmt.Println("  codes notify add <url>")
		return
	}

	fmt.Printf("Configured Webhooks (%d):\n\n", len(webhooks))
	for i, w := range webhooks {
		if w.Name != "" {
			fmt.Printf("%d. %s\n", i+1, w.Name)
		} else {
			fmt.Printf("%d. (unnamed)\n", i+1)
		}
		fmt.Printf("   URL: %s\n", w.URL)
		fmt.Printf("   Format: %s\n", w.Format)
		if len(w.Events) > 0 {
			fmt.Printf("   Events: %s\n", strings.Join(w.Events, ", "))
		} else {
			fmt.Printf("   Events: all\n")
		}
		if len(w.Extra) > 0 {
			fmt.Printf("   Extra: %s\n", formatExtra(w.Extra))
		}
		fmt.Println()
	}
}

// RunNotifyTest tests a webhook by sending a test notification.
func RunNotifyTest(identifier string) {
	webhook, ok := config.GetWebhook(identifier)
	if !ok {
		ui.ShowError("Webhook not found: "+identifier, nil)
		return
	}

	ui.ShowInfo("Testing webhook: %s", webhook.URL)

	notifier := notify.NewWebhookNotifier(webhook.URL, webhook.Format, webhook.Extra)
	err := notifier.Send(notify.Notification{
		Title:   "codes test notification",
		Message: "This is a test notification from codes CLI",
		Sound:   false,
	})

	if err != nil {
		if output.JSONMode {
			printJSON(map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		ui.ShowError("Webhook test failed", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"webhook": identifier,
		})
		return
	}

	ui.ShowSuccess("Webhook test successful!")
}

// RunHookSet sets a shell hook for the given event.
func RunHookSet(event, scriptPath string) {
	if err := config.SetHook(event, scriptPath); err != nil {
		ui.ShowError("Failed to set hook", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"event":   event,
			"script":  scriptPath,
		})
		return
	}

	ui.ShowSuccess("Hook set for %s", event)
	fmt.Printf("  Script: %s\n", scriptPath)
}

// RunHookRemove removes the shell hook for the given event.
func RunHookRemove(event string) {
	if err := config.RemoveHook(event); err != nil {
		ui.ShowError("Failed to remove hook", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"removed": event,
		})
		return
	}

	ui.ShowSuccess("Hook removed for %s", event)
}

// RunHookList lists all configured shell hooks.
func RunHookList() {
	hooks := config.ListHooks()

	if output.JSONMode {
		printJSON(map[string]any{
			"hooks": hooks,
			"count": len(hooks),
		})
		return
	}

	if len(hooks) == 0 {
		fmt.Println("No hooks configured")
		fmt.Println("\nSet a hook with:")
		fmt.Println("  codes notify hook set <event> <script-path>")
		fmt.Println("\nAvailable events: on_task_completed, on_task_failed")
		return
	}

	fmt.Printf("Configured Hooks (%d):\n\n", len(hooks))
	for event, script := range hooks {
		fmt.Printf("  %s → %s\n", event, script)
	}
}

// RunHookTest tests a hook by executing it with a mock payload.
func RunHookTest(event string) {
	scriptPath := config.GetHook(event)
	if scriptPath == "" {
		ui.ShowError("No hook configured for event: "+event, nil)
		return
	}

	ui.ShowInfo("Testing hook for %s: %s", event, scriptPath)

	payload := notify.HookPayload{
		Team:      "test-team",
		TaskID:    0,
		Subject:   "Test task (hook test)",
		Status:    strings.TrimPrefix(event, "on_task_"),
		Agent:     "test-agent",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if event == "on_task_completed" {
		payload.Result = "Test result from hook test"
	} else {
		payload.Error = "Test error from hook test"
	}

	runner := notify.NewHookRunner(scriptPath)
	err := runner.Execute(payload)

	if err != nil {
		if output.JSONMode {
			printJSON(map[string]any{
				"success": false,
				"event":   event,
				"error":   err.Error(),
			})
			return
		}
		ui.ShowError("Hook test failed", err)
		return
	}

	if output.JSONMode {
		printJSON(map[string]any{
			"success": true,
			"event":   event,
		})
		return
	}

	ui.ShowSuccess("Hook test successful!")
}

// formatExtra formats a map as "key=value, key=value".
func formatExtra(extra map[string]string) string {
	parts := make([]string, 0, len(extra))
	for k, v := range extra {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(parts, ", ")
}
