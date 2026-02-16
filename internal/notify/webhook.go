package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

// WebhookNotifier sends notifications to a webhook URL.
type WebhookNotifier struct {
	URL    string            // webhook endpoint
	Format string            // "slack", "feishu", "dingtalk", "telegram", "custom"
	Extra  map[string]string // format-specific parameters (e.g. chat_id, template)
	client *http.Client
}

// NewWebhookNotifier creates a webhook notifier for the given URL, format, and extra parameters.
func NewWebhookNotifier(url, format string, extra map[string]string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    url,
		Format: format,
		Extra:  extra,
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
	case "dingtalk":
		payload = map[string]any{
			"msgtype": "text",
			"text": map[string]string{
				"content": text,
			},
		}
	case "telegram":
		payload = map[string]any{
			"chat_id":    w.Extra["chat_id"],
			"text":       text,
			"parse_mode": "HTML",
		}
	case "custom":
		tmplStr := w.Extra["template"]
		if tmplStr == "" {
			return fmt.Errorf("webhook custom format: missing 'template' in extra")
		}
		tmpl, err := template.New("webhook").Parse(tmplStr)
		if err != nil {
			return fmt.Errorf("webhook custom template parse: %w", err)
		}
		data := map[string]string{
			"Title":   n.Title,
			"Message": n.Message,
			"Text":    text,
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("webhook custom template execute: %w", err)
		}
		// Parse rendered template as JSON to validate it
		if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
			return fmt.Errorf("webhook custom template produced invalid JSON: %w", err)
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
