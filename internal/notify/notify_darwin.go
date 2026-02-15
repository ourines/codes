//go:build darwin

package notify

import (
	"fmt"
	"os/exec"
)

type darwinNotifier struct{}

func newPlatformNotifier() Notifier {
	return &darwinNotifier{}
}

func (d *darwinNotifier) Send(n Notification) error {
	script := fmt.Sprintf(`display notification %q with title %q`, n.Message, n.Title)
	if n.Sound {
		script += ` sound name "default"`
	}
	return exec.Command("osascript", "-e", script).Run()
}

func (d *darwinNotifier) Name() string { return "darwin" }
