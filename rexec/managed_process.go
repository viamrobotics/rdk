package rexec

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/edaniels/golog"
)

func NewManagedProcess(config ProcessConfig, logger golog.Logger) *ManagedProcess {
	return &ManagedProcess{
		name:      config.Name,
		args:      config.Args,
		cwd:       config.CWD,
		oneShot:   config.OneShot,
		shouldLog: config.Log,
		logger:    logger,
	}
}

type ManagedProcess struct {
	mu sync.Mutex

	// TODO(erd): probably need a unique identifier for
	// live management
	name      string
	args      []string
	cwd       string
	oneShot   bool
	shouldLog bool
	cmd       *exec.Cmd

	stopped  bool
	managing chan struct{}
	killCh   chan struct{}

	logger golog.Logger
}

func (p *ManagedProcess) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.oneShot {
		cmd := exec.CommandContext(ctx, p.name, p.args...)
		cmd.Dir = p.cwd
		if !p.shouldLog {
			out, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("error running process %q: %w", p.name, err)
			}
			p.logger.Debugw("process output", "name", p.name, "output", string(out))
			return nil
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error running process %q: %w", p.name, err)
		}
		return nil
	}

	// this is fully managed so we will control when to kill the process
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
	p.cmd = cmd // okay to gc old cmd

	p.managing = make(chan struct{})
	p.killCh = make(chan struct{})
	go p.manage(stdOut, stdErr)
	return nil
}

func (p *ManagedProcess) manage(stdOut, stdErr io.ReadCloser) {
	var restarting bool
	defer func() {
		if !restarting {
			close(p.managing)
		}
	}()

	stopLogging := make(chan struct{})
	var activeLoggers sync.WaitGroup
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
				if err != io.EOF {
					p.logger.Errorw("error reading output", "name", name, "error", err)
				}
				return
			}
			p.logger.Debugw("output", "name", name, "data", string(line))
		}
	}
	if p.shouldLog {
		activeLoggers.Add(2)
		go logPipe("StdOut", stdOut)
		go logPipe("StdErr", stdErr)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.killCh:
			return
		case <-ticker.C:
		}
		state, err := p.cmd.Process.Wait()

		// possible that Stop was called, so check killCh and return early
		select {
		case <-p.killCh:
			return
		default:
		}
		if err != nil {
			p.logger.Errorw("error waiting for process during manage", "error", err)
			continue
		}

		if !state.Exited() {
			p.logger.Debugw("oddity; wait finished but process did not exit", "state", state)
			continue
		}
		p.logger.Infow("process exited before expected", "code", state.ExitCode())
		p.logger.Info("restarting process")
		restarting = true
		close(stopLogging)
		activeLoggers.Wait()
		if err := p.Start(context.Background()); err != nil {
			p.logger.Errorw("error restarting process; will try again", "error", err)
			continue
		}
		return
	}
}

const waitInterrupt = 3 * time.Second

func (p *ManagedProcess) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		return nil
	}
	p.stopped = true

	if p.cmd == nil {
		return nil
	}
	close(p.killCh)
	p.logger.Info("stopping")
	waited := make(chan struct{})
	var state *os.ProcessState
	var waitErr error
	go func() {
		state, waitErr = p.cmd.Process.Wait()
		close(waited)
	}()
	if err := p.cmd.Process.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("error interrupting process: %w", err)
	}
	timer := time.NewTimer(waitInterrupt)
	defer timer.Stop()
	select {
	case <-timer.C:
		if err := p.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("error killing process: %w", err)
		}
	case <-waited:
	}
	<-waited
	<-p.managing

	if waitErr != nil {
		return waitErr
	}
	if !state.Success() {
		return fmt.Errorf("non-successful exit code: %d", state.ExitCode())
	}
	return nil
}
