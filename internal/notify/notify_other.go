//go:build !darwin && !linux && !windows

package notify

// noopNotifier is a no-op for unsupported platforms.
type noopNotifier struct{}

func newPlatformNotifier() Notifier {
	return &noopNotifier{}
}

func (n *noopNotifier) Send(_ Notification) error { return nil }
func (n *noopNotifier) Name() string               { return "noop" }
