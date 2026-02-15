package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookNotifier sends notifications to a webhook URL.
type WebhookNotifier struct {
	URL    string // webhook endpoint
	Format string // "slack" or "feishu"
	client *http.Client
}

// NewWebhookNotifier creates a webhook notifier for the given URL and format.
func NewWebhookNotifier(url, format string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    url,
		Format: format,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts the notification to the configured webhook.
func (w *WebhookNotifier) Send(n Notification) error {
	var payload any

	text := fmt.Sprintf("%s: %s", n.Title, n.Message)

	switch w.Format {
	case "feishu":
		payload = map[string]any{
			"msg_type": "text",
			"content": map[string]string{
				"text": text,
			},
		}
	default: // "slack" and any other format
		payload = map[string]string{
			"text": text,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("webhook marshal: %w", err)
	}

	resp, err := w.client.Post(w.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// Name returns the name of this notifier.
func (w *WebhookNotifier) Name() string { return "webhook" }
