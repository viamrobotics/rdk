package modmanager

import (
	"os"
	"syscall"
)

func kill(pid int, _ syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
