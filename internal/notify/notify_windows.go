//go:build windows

package notify

import (
	"fmt"
	"log"
	"os/exec"
)

type windowsNotifier struct{}

func newPlatformNotifier() Notifier {
	return &windowsNotifier{}
}

func (w *windowsNotifier) Send(n Notification) error {
	// Use PowerShell to show a toast notification via .NET APIs.
	// This works on Windows 10+ without extra dependencies.
	script := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$textNodes = $template.GetElementsByTagName("text")
$textNodes.Item(0).AppendChild($template.CreateTextNode(%q)) > $null
$textNodes.Item(1).AppendChild($template.CreateTextNode(%q)) > $null
$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("codes").Show($toast)
`, n.Title, n.Message)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	if err := cmd.Run(); err != nil {
		log.Printf("notify: Windows toast failed, skipping: %v", err)
		return nil // graceful degradation
	}
	return nil
}

func (w *windowsNotifier) Name() string { return "windows" }
