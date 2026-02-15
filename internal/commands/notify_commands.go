package commands

import (
	"codes/internal/config"
	"codes/internal/notify"
	"codes/internal/output"
	"codes/internal/ui"
	"fmt"
	"strings"
)

// RunNotifyAdd adds a webhook configuration.
func RunNotifyAdd(url, name, format string, events []string) {
	// Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		ui.ShowError("Invalid webhook URL: must start with http:// or https://", nil)
		return
	}

	// Validate format
	if format != "slack" && format != "feishu" {
		ui.ShowError("Invalid format: must be 'slack' or 'feishu'", nil)
		return
	}

	webhook := config.WebhookConfig{
		Name:   name,
		URL:    url,
		Format: format,
		Events: events,
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

	notifier := notify.NewWebhookNotifier(webhook.URL, webhook.Format)
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
