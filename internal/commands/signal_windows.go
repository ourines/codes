//go:build windows

package commands

import (
	"os"
	"os/signal"
)

// notifySignals registers the given channel to receive interrupt signals.
// On Windows, only os.Interrupt is available (SIGTERM is not supported).
func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt)
}
