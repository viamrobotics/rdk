//go:build !windows

package modmanager

import "syscall"
import "braces.dev/errtrace"

func kill(pid int, signal syscall.Signal) error {
	return errtrace.Wrap(syscall.Kill(pid, signal))
}
