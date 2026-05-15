//go:build !windows

package modmanager

import "syscall"

func kill(pid int, signal syscall.Signal) error {
	return syscall.Kill(pid, signal)
}
