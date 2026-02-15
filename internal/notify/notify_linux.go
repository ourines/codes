//go:build linux

package notify

import (
	"log"
	"os/exec"
)

type linuxNotifier struct{}

func newPlatformNotifier() Notifier {
	return &linuxNotifier{}
}

func (l *linuxNotifier) Send(n Notification) error {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		log.Printf("notify: notify-send not found, skipping desktop notification")
		return nil
	}

	args := []string{n.Title, n.Message}
	if n.Sound {
		args = append(args, "--hint=string:sound-name:message-new-instant")
	}
	return exec.Command(path, args...).Run()
}

func (l *linuxNotifier) Name() string { return "linux" }
