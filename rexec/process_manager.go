package rexec

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

type ProcessManager interface {
	Start(ctx context.Context) error
	AddProcess(config ProcessConfig)
	Stop() error
}

type processManager struct {
	mu        sync.Mutex
	processes []*ManagedProcess
	logger    golog.Logger
}

func NewProcessManager(logger golog.Logger) ProcessManager {
	return &processManager{logger: logger}
}

func (pm *processManager) Start(ctx context.Context) error {
	for _, proc := range pm.processes {
		if err := proc.Start(ctx); err != nil {
			return multierr.Combine(err, pm.Stop())
		}
	}
	return nil
}

func (pm *processManager) AddProcess(config ProcessConfig) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	logger := pm.logger.Named(fmt.Sprintf("process.%s", config.Name))
	proc := NewManagedProcess(config, logger)
	pm.processes = append(pm.processes, proc)
}

func (pm *processManager) Stop() error {
	var err error
	for _, proc := range pm.processes {
		err = multierr.Combine(err, proc.Stop())
	}
	return err
}
