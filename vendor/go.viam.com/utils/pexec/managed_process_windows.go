//go:build windows

package pexec

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
)

func sigStr(sig syscall.Signal) string {
	return "<UNKNOWN>"
}

var knownSignals []syscall.Signal = nil

func parseSignal(sigStr, name string) (syscall.Signal, error) {
	if sigStr == "" {
		return 0, nil
	}
	return 0, errors.New("signals not supported on Windows")
}

func (p *managedProcess) sysProcAttr() (*syscall.SysProcAttr, error) {
	ret := &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if len(p.username) > 0 {
		return nil, errors.Errorf("can't run as user %s, not supported yet on windows", p.username)
	}
	return ret, nil
}

func (p *managedProcess) kill() (bool, error) {
	const mustForce = "This process can only be terminated forcefully"
	pidStr := strconv.Itoa(p.cmd.Process.Pid)
	p.logger.Infof("killing process %d", p.cmd.Process.Pid)
	// First let's try to ask the process to stop. If it's a console application, this is
	// very unlikely to work.
	var shouldJustForce bool
	if out, err := exec.Command("taskkill", "/pid", pidStr).CombinedOutput(); err != nil {
		switch {
		case strings.Contains(string(out), mustForce):
			p.logger.Debug("must force terminate process")
			shouldJustForce = true
		case strings.Contains(string(out), "not found"):
			return false, nil
		default:
			return false, errors.Wrapf(err, "error killing process %d", p.cmd.Process.Pid)
		}
	}

	if !shouldJustForce {
		// In case the process didn't stop, or left behind any orphan children in its process group,
		// we now ask everything in the process tree to stop after a brief wait.
		timer := time.NewTimer(p.stopWaitInterval)
		defer timer.Stop()
		select {
		case <-timer.C:
			p.logger.Infof("killing entire process tree %d", p.cmd.Process.Pid)
			if out, err := exec.Command("taskkill", "/t", "/pid", pidStr).CombinedOutput(); err != nil {
				switch {
				case strings.Contains(string(out), mustForce):
					p.logger.Debug("must force terminate process tree")
					shouldJustForce = true
				case strings.Contains(string(out), "not found"):
					return false, nil
				default:
					return false, errors.Wrapf(err, "error killing process tree %d", p.cmd.Process.Pid)
				}
			}
		case <-p.managingCh:
			timer.Stop()
		}
	}

	// Lastly, kill everything in the process tree that remains after a longer wait or now. This is
	// going to likely result in an "exit status 1" that we will have to interpret.
	// FUTURE(erd): find a way to do this better. Research has not come up with much and is
	// program dependent.
	var forceKilled bool
	if !shouldJustForce {
		timer2 := time.NewTimer(p.stopWaitInterval * 2)
		defer timer2.Stop()
		select {
		case <-timer2.C:
			p.logger.Infof("force killing entire process tree %d", p.cmd.Process.Pid)
			if err := exec.Command("taskkill", "/t", "/f", "/pid", pidStr).Run(); err != nil {
				return false, errors.Wrapf(err, "error force killing process tree %d", p.cmd.Process.Pid)
			}
			forceKilled = true
		case <-p.managingCh:
			timer2.Stop()
		}
	} else {
		if err := exec.Command("taskkill", "/t", "/f", "/pid", pidStr).Run(); err != nil {
			return false, errors.Wrapf(err, "error force killing process tree %d", p.cmd.Process.Pid)
		}
		forceKilled = true
	}

	return forceKilled, nil
}

// forceKillGroup kills everything in the process tree. This will not wait for completion and may result in a zombie process.
func (p *managedProcess) forceKillGroup() error {
	pidStr := strconv.Itoa(p.cmd.Process.Pid)
	p.logger.Infof("force killing entire process tree %d", p.cmd.Process.Pid)
	return exec.Command("taskkill", "/t", "/f", "/pid", pidStr).Start()
}

func isWaitErrUnknown(err string, forceKilled bool) bool {
	if !forceKilled {
		return false
	}
	// when we force kill, it's very easy to get 1.
	return err == "exit status 1"
}
