package pexec

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/utils"
)

// A ProcessManager is responsible for controlling the lifecycle of processes
// added to it.
type ProcessManager interface {
	// ProcessIDs returns the IDs of all managed processes.
	ProcessIDs() []string

	// ProcessByID fetches the process by the given ID if it exists.
	ProcessByID(id string) (ManagedProcess, bool)

	// RemoveProcessByID removes a managed process by the given ID if it exists.
	// It does not stop it it.
	RemoveProcessByID(id string) (ManagedProcess, bool)

	// Start starts all added processes and errors if any fail to start. The
	// given context is only used for one shot processes.
	Start(ctx context.Context) error

	// AddProcess manages the given process and potentially starts it depending
	// on the state of the ProcessManager and if it's requested. The same context
	// semantics in Start apply here. If the process is replaced by its ID, the
	// replaced process will be returned.
	AddProcess(ctx context.Context, proc ManagedProcess, start bool) (ManagedProcess, error)

	// AddProcess manages a new process from the given configuration and
	// potentially starts it depending on the state of the ProcessManager.
	// The same context semantics in Start apply here. If the process is
	// replaced by its ID, the replaced process will be returned.
	AddProcessFromConfig(ctx context.Context, config ProcessConfig) (ManagedProcess, error)

	// Stop signals and waits for all managed processes to stop and returns
	// any errors from stopping them.
	Stop() error

	// Clone gives a copy of the processes being managed but provides
	// no guarantee of the current state of the processes.
	Clone() ProcessManager
}

type processManager struct {
	mu            sync.Mutex
	processesByID map[string]ManagedProcess
	logger        utils.ZapCompatibleLogger
	started       bool
	stopped       bool
}

// NewProcessManager returns a new ProcessManager.
func NewProcessManager(logger utils.ZapCompatibleLogger) ProcessManager {
	return &processManager{
		logger:        logger,
		processesByID: map[string]ManagedProcess{},
	}
}

func (pm *processManager) ProcessIDs() []string {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	ids := make([]string, 0, len(pm.processesByID))
	for id := range pm.processesByID {
		ids = append(ids, id)
	}
	return ids
}

func (pm *processManager) ProcessByID(id string) (ManagedProcess, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stopped {
		return nil, false
	}
	proc, ok := pm.processesByID[id]
	return proc, ok
}

func (pm *processManager) RemoveProcessByID(id string) (ManagedProcess, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stopped {
		return nil, false
	}
	proc, ok := pm.processesByID[id]
	if !ok {
		return nil, false
	}
	delete(pm.processesByID, id)
	return proc, true
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
	for _, proc := range pm.processesByID {
		if err := proc.Start(ctx); err != nil {
			// be sure to stop anything that has already started
			return multierr.Combine(err, pm.stop())
		}
	}
	pm.started = true
	return nil
}

func (pm *processManager) AddProcess(ctx context.Context, proc ManagedProcess, start bool) (ManagedProcess, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stopped {
		return nil, errAlreadyStopped
	}
	replaced := pm.processesByID[proc.ID()]
	if pm.started && start {
		if err := proc.Start(ctx); err != nil {
			return nil, err
		}
	}
	pm.processesByID[proc.ID()] = proc
	return replaced, nil
}

func (pm *processManager) AddProcessFromConfig(ctx context.Context, config ProcessConfig) (ManagedProcess, error) {
	if pm.stopped {
		return nil, errAlreadyStopped
	}
	proc := NewManagedProcess(config, pm.logger)
	return pm.AddProcess(ctx, proc, true)
}

func (pm *processManager) Stop() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.stopped {
		return nil
	}
	return pm.stop()
}

func (pm *processManager) stop() error {
	pm.stopped = true
	var err error
	for _, proc := range pm.processesByID {
		err = multierr.Combine(err, proc.Stop())
	}
	pm.processesByID = nil
	return err
}

func (pm *processManager) Clone() ProcessManager {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	processesByIDCopy := make(map[string]ManagedProcess, len(pm.processesByID))
	for k, v := range pm.processesByID {
		processesByIDCopy[k] = v
	}
	return &processManager{
		processesByID: processesByIDCopy,
		logger:        pm.logger,
		started:       pm.started,
		stopped:       pm.stopped,
	}
}

// MergeAddProcessManagers merges in another process manager and takes ownership of
// its processes. This may replace existing processes and it's the
// callers responsibility to stop what has been replaced.
func MergeAddProcessManagers(dst, src ProcessManager) ([]ManagedProcess, error) {
	var replacements []ManagedProcess
	ids := src.ProcessIDs()
	for _, id := range ids {
		proc, ok := src.ProcessByID(id)
		if !ok {
			continue // should not happen
		}
		replaced, err := dst.AddProcess(context.Background(), proc, false)
		if err != nil {
			return nil, err
		}
		if replaced != nil {
			replacements = append(replacements, replaced)
		}
	}
	return replacements, nil
}

// MergeRemoveProcessManagers merges in another process manager and removes ownership of
// its own processes. It does not stop the processes.
func MergeRemoveProcessManagers(dst, src ProcessManager) []ManagedProcess {
	ids := src.ProcessIDs()
	removed := make([]ManagedProcess, 0, len(ids))
	for _, id := range ids {
		proc, ok := dst.RemoveProcessByID(id)
		if !ok {
			continue // should not happen
		}
		removed = append(removed, proc)
	}
	return removed
}

type noopProcessManager struct{}

// NoopProcessManager does nothing and is useful for places that
// need to return some ProcessManager.
var NoopProcessManager = &noopProcessManager{}

func (noop noopProcessManager) ProcessIDs() []string {
	return nil
}

func (noop noopProcessManager) ProcessByID(id string) (ManagedProcess, bool) {
	return nil, false
}

func (noop noopProcessManager) RemoveProcessByID(id string) (ManagedProcess, bool) {
	return nil, false
}

func (noop noopProcessManager) Start(ctx context.Context) error {
	return nil
}

func (noop noopProcessManager) AddProcess(ctx context.Context, proc ManagedProcess, start bool) (ManagedProcess, error) {
	return nil, errors.New("unsupported")
}

func (noop noopProcessManager) AddProcessFromConfig(ctx context.Context, config ProcessConfig) (ManagedProcess, error) {
	return nil, errors.New("unsupported")
}

func (noop noopProcessManager) Stop() error {
	return nil
}

func (noop noopProcessManager) Clone() ProcessManager {
	return noop
}
