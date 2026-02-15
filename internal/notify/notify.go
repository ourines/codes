package notify

import "strings"

// Notification represents a notification to be sent.
type Notification struct {
	Title   string
	Message string
	Sound   bool
}

// Notifier sends notifications.
type Notifier interface {
	Send(n Notification) error
	Name() string
}

// NewDesktopNotifier returns a platform-specific desktop notification sender.
func NewDesktopNotifier() Notifier {
	return newPlatformNotifier()
}

// MultiNotifier sends notifications to multiple notifiers concurrently.
type MultiNotifier struct {
	notifiers []Notifier
}

// NewMultiNotifier creates a MultiNotifier from the given notifiers.
func NewMultiNotifier(ns ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: ns}
}

// Send dispatches the notification to all registered notifiers.
// Returns the first error encountered, but attempts all notifiers.
func (m *MultiNotifier) Send(n Notification) error {
	var firstErr error
	for _, notifier := range m.notifiers {
		if err := notifier.Send(n); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Name returns the name of this notifier.
func (m *MultiNotifier) Name() string {
	names := make([]string, len(m.notifiers))
	for i, n := range m.notifiers {
		names[i] = n.Name()
	}
	return "multi(" + strings.Join(names, ",") + ")"
}
