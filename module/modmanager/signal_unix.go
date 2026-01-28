//go:build unix

package modmanager

import (
	"syscall"
)

// requestStackTraceDump sends SIGUSR1 to the module process to request a stack trace dump.
func (m *module) requestStackTraceDump() {
	if m.process == nil {
		return
	}
	pid, err := m.process.UnixPid()
	if err != nil {
		m.logger.Warnw("Failed to get module PID for stack trace request", "module", m.cfg.Name, "error", err)
		return
	}
	m.logger.Infof("Requesting stack trace dump from module: %s (PID %d)", m.cfg.Name, pid)
	if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
		m.logger.Warnw("Failed to send SIGUSR1 to module", "module", m.cfg.Name, "pid", pid, "error", err)
	}
}
