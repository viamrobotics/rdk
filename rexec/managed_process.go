package rexec

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/edaniels/golog"

	"go.viam.com/core/utils"
)

var errAlreadyStopped = errors.New("already stopped")

// waitInterrupt is how long to wait after interrupting to move onto killing.
const waitInterrupt = 3 * time.Second

// A ManagedProcess controls the lifecycle of a single system process. Based on
// its configuration, it will ensure the process is revived if it every unexpectedly
// perishes.
type ManagedProcess interface {
	// ID returns the unique ID of the process.
	ID() string

	// Start starts the process. The given context is only used for one shot processes.
	Start(ctx context.Context) error

	// Stop signals and waits for the process to stop. An error is returned if
	// there's any system level issue stopping the process.
	Stop() error
}

// NewManagedProcess returns a new, unstarted, from the given configuration.
func NewManagedProcess(config ProcessConfig, logger golog.Logger) ManagedProcess {
	logger = logger.Named(fmt.Sprintf("process.%s_%s", config.ID, config.Name))
	return &managedProcess{
		id:         config.ID,
		name:       config.Name,
		args:       config.Args,
		cwd:        config.CWD,
		oneShot:    config.OneShot,
		shouldLog:  config.Log,
		managingCh: make(chan struct{}),
		killCh:     make(chan struct{}),
		logger:     logger,
	}
}

type managedProcess struct {
	mu sync.Mutex

	id        string
	name      string
	args      []string
	cwd       string
	oneShot   bool
	shouldLog bool
	cmd       *exec.Cmd

	stopped     bool
	managingCh  chan struct{}
	killCh      chan struct{}
	lastWaitErr error

	logger golog.Logger
}

func (p *managedProcess) ID() string {
	return p.id
}

func (p *managedProcess) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// In the event this Start happened from a restart but a
	// stop happened while we were acquiring the lock, we may
	// need to return early.
	select {
	case <-p.killCh:
		// This will signal to a potential restarter that
		// there's no restart to do.
		return errAlreadyStopped
	default:
	}

	if p.oneShot {
		// Here we use the context since we block on waiting for the command
		// to finish running.
		cmd := exec.CommandContext(ctx, p.name, p.args...)
		cmd.Dir = p.cwd
		var runErr error
		if p.shouldLog {
			out, err := cmd.CombinedOutput()
			if len(out) > 0 {
				p.logger.Debugw("process output", "name", p.name, "output", string(out))
			}
			if err != nil {
				runErr = err
			}
		} else {
			runErr = cmd.Run()
		}
		if runErr == nil {
			return nil
		}
		return fmt.Errorf("error running process %q: %w", p.name, runErr)
	}

	// This is fully managed so we will control when to kill the process and not
	// use the CommandContext variant.
	cmd := exec.Command(p.name, p.args...)
	cmd.Dir = p.cwd

	var stdOut, stdErr io.ReadCloser
	if p.shouldLog {
		var err error
		stdOut, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stdErr, err = cmd.StderrPipe()
		if err != nil {
			return err
		}
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// We have the lock here so it's okay to:
	// 1. Unset the old command, if there was one and let it be GC'd.
	// 2. Assign a new command to be referenced in other places.
	p.cmd = cmd

	// It's okay to not wait for management to start.
	utils.ManagedGo(func() {
		p.manage(stdOut, stdErr)
	}, nil)
	return nil
}

// manage is the watchdog of the process. Any time it detects
// the process has ended unexpectedly, it will restart it. It's
// possible and okay for a restart to be in progress while a Stop
// is happening. As a means simplifying implementation, a restart
// spawns new goroutines by calling Start again and lets the original
// goroutine die off.
func (p *managedProcess) manage(stdOut, stdErr io.ReadCloser) {
	// If no restart is going to happen after this function exits,
	// then we want to notify anyone listening that this process
	// is done being managed. We assume that if we aren't managing,
	// the process is no longer running (it could have double forked though).
	var restarted bool
	defer func() {
		if !restarted {
			close(p.managingCh)
		}
	}()

	// This block here logs as much as possible if it's requested until the
	// pipes are closed.
	stopLogging := make(chan struct{})
	var activeLoggers sync.WaitGroup
	if p.shouldLog {
		logPipe := func(name string, pipe io.ReadCloser) {
			defer activeLoggers.Done()
			pipeR := bufio.NewReader(pipe)
			for {
				select {
				case <-stopLogging:
					return
				default:
				}
				line, _, err := pipeR.ReadLine()
				if err != nil {
					if !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
						p.logger.Errorw("error reading output", "name", name, "error", err)
					}
					return
				}
				p.logger.Debugw("output", "name", name, "data", string(line))
			}
		}
		activeLoggers.Add(2)
		utils.PanicCapturingGo(func() {
			logPipe("StdOut", stdOut)
		})
		utils.PanicCapturingGo(func() {
			logPipe("StdErr", stdErr)
		})
	}

	err := p.cmd.Wait()
	// This is safe to write to because it is only read in Stop which
	// is waiting for us to stop managing.
	if err == nil {
		p.lastWaitErr = nil
	} else {
		p.lastWaitErr = err
	}
	close(stopLogging)
	activeLoggers.Wait()

	// It's possible that Stop was called and is the reason why Wait returned.
	select {
	case <-p.killCh:
		return
	default:
	}

	// Otherwise, let's try restarting the process.
	if err != nil {
		// Right now we are assuming that any wait error implies the process is no longer
		// alive. TODO(https://github.com/viamrobotics/robotcore/issues/46): Verify that
		// this is actually true. If it's false, we could be multiply spawning processes
		// where all are orphaned but one.
		p.logger.Errorw("error waiting for process during manage", "error", err)
	}

	if p.cmd.ProcessState.Exited() {
		p.logger.Infow("process exited before expected", "code", p.cmd.ProcessState.ExitCode())
	} else {
		p.logger.Infow("process exited before expected", "state", p.cmd.ProcessState)
	}
	p.logger.Info("restarting process")

	// Temper ourselves so we aren't constantly restarting if we immediately fail.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	select {
	case <-ticker.C:
	case <-p.killCh:
		return
	}

	err = p.Start(context.Background())
	if err != nil {
		if !errors.Is(err, errAlreadyStopped) {
			// MAYBE(erd): add retry
			p.logger.Errorw("error restarting process", "error", err)
		}
		return
	}
	restarted = true
}

func (p *managedProcess) Stop() error {
	// Minimally hold a lock here so that we can signal the
	// management goroutine to stop. If we were to hold the
	// lock for the duration of the function, we would possibly
	// deadlock with manage trying to restart.
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return nil
	}
	close(p.killCh)
	p.stopped = true

	if p.cmd == nil {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	// Since p.cmd is mutex guarded and we just signaled the manage
	// goroutine to stop, no new Start can happen and therefore
	// p.cmd can no longer be modified rendering it safe to read
	// without a lock held.

	p.logger.Info("stopping")
	// First let's try to interrupt the process.
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("error interrupting process: %w", err)
	}

	// If after a while the process still isn't stopping, let's kill it.
	timer := time.NewTimer(waitInterrupt)
	defer timer.Stop()
	select {
	case <-timer.C:
		if err := p.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("error killing process: %w", err)
		}
	case <-p.managingCh:
	}
	<-p.managingCh

	if p.lastWaitErr == nil && p.cmd.ProcessState.Success() {
		return nil
	}

	if p.lastWaitErr != nil {
		var unknownStatus bool
		var errno syscall.Errno
		if errors.As(p.lastWaitErr, &errno) {
			// We lost the race to wait before the signal was caught. We're
			// not going to be able to report any information here about the
			// process stopping, unfortunately.
			if errno == syscall.ECHILD {
				unknownStatus = true
			}
		}

		// This can easily happen if the process does not handle interrupts gracefully
		// and it won't provide us any exit code info.
		switch p.lastWaitErr.Error() {
		case "signal: interrupt", "signal: killed":
			unknownStatus = true
		}
		if unknownStatus {
			p.logger.Debug("unable to check exit status")
			return nil
		}
		return p.lastWaitErr
	}
	return fmt.Errorf("non-successful exit code: %d", p.cmd.ProcessState.ExitCode())
}
