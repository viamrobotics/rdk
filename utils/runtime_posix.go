//go:build !windows

package utils

import (
	"os"
	"os/signal"
	"syscall"
)

// copied from goutils
func notifySignals(channel chan os.Signal) {
	signal.Notify(channel, syscall.SIGUSR1)
}
