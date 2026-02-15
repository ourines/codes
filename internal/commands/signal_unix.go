//go:build !windows

package commands

import (
	"os"
	"os/signal"
	"syscall"
)

// notifySignals registers the given channel to receive interrupt and
// termination signals. On Unix-like systems SIGTERM is included.
func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}
