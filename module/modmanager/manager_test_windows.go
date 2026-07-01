package modmanager

import (
	"braces.dev/errtrace"
	"os"
	"syscall"
)

func kill(pid int, _ syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return errtrace.Wrap(err)
	}
	return errtrace.Wrap(p.Kill())
}
