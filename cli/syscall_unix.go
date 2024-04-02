//go:build unix
// +build unix

package cli

import (
	"os"
	"syscall"
)

func sigwinchSignal() (os.Signal, bool) {
	return syscall.SIGWINCH, true
}
