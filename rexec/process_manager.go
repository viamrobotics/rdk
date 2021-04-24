package rexec

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// A ProcessManager is responsible for controlling the lifecycle of processes
// added to it.
type ProcessManager interface {
	// Start starts all added processes and errors if any fail to start. The
	// given context is only used for one shot processes.
	Start(ctx context.Context) error

	// AddProcess manages the given process and potentially starts it depending
	// on the state of the ProcessManager. The same context semantics in
	// Start apply here.
	AddProcess(ctx context.Context, proc ManagedProcess) error

	// AddProcess manages a new process from the given configuration and
	// potentially starts it depending on the state of the ProcessManager.
	// The same context semantics in Start apply here.
	AddProcessFromConfig(ctx context.Context, config ProcessConfig) error

	// Stop signals and waits for all managed processes to stop and returns
	// any errors from stopping them.
	Stop() error
}

type processManager struct {
	mu        sync.Mutex
	processes []ManagedProcess
	logger    golog.Logger
	started   bool
	stopped   bool
}

// NewProcessManager returns a new ProcessManager.
func NewProcessManager(logger golog.Logger) ProcessManager {
	return &processManager{logger: logger}
}

func (pm *processManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stopped {
		return errAlreadyStopped
	}
	if pm.started {
		return nil
	}
	for _, proc := range pm.processes {
		if err := proc.Start(ctx); err != nil {
			// be sure to stop anything that has already started
			return multierr.Combine(err, pm.stop())
		}
	}
	pm.started = true
	return nil
}

func (pm *processManager) AddProcess(ctx context.Context, proc ManagedProcess) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.started {
		if err := proc.Start(ctx); err != nil {
			return err
		}
	}
	pm.processes = append(pm.processes, proc)
	return nil
}

func (pm *processManager) AddProcessFromConfig(ctx context.Context, config ProcessConfig) error {
	proc := NewManagedProcess(config, pm.logger)
	return pm.AddProcess(ctx, proc)
}

func (pm *processManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if !pm.started {
		return nil
	}
	if pm.stopped {
		return nil
	}
	return pm.stop()
}

func (pm *processManager) stop() error {
	pm.stopped = true
	var err error
	for _, proc := range pm.processes {
		err = multierr.Combine(err, proc.Stop())
	}
	pm.processes = nil
	return err
}
