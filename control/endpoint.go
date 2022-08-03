package control

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type endpoint struct {
	mu     sync.Mutex
	ctr    Controllable
	cfg    ControlBlockConfig
	y      []Signal
	logger golog.Logger
}

func newEndpoint(config ControlBlockConfig, logger golog.Logger, ctr Controllable) (ControlBlock, error) {
	e := &endpoint{cfg: config, logger: logger, ctr: ctr}
	if err := e.reset(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *endpoint) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) == 1 {
		power := x[0].GetSignalValueAt(0)
		if e.ctr != nil {
			err := e.ctr.SetPower(ctx, power, nil)
			if err != nil {
				return []Signal{}, false
			}
		}
		return []Signal{}, false
	}
	if len(x) == 0 {
		if e.ctr != nil {
			pos, err := e.ctr.GetPosition(ctx, nil)
			if err != nil {
				return []Signal{}, false
			}
			e.y[0].SetSignalValueAt(0, pos)
		}
		return e.y, true
	}
	return e.y, false
}

func (e *endpoint) reset() error {
	if !e.cfg.Attribute.Has("motor_name") {
		return errors.Errorf("endpoint %s should have a motor_name field", e.cfg.Name)
	}
	e.y = make([]Signal, 1)
	e.y[0] = makeSignal(e.cfg.Name, 1)
	return nil
}

func (e *endpoint) Reset(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reset()
}

func (e *endpoint) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = config
	return e.reset()
}

func (e *endpoint) Output(ctx context.Context) []Signal {
	return e.y
}

func (e *endpoint) Config(ctx context.Context) ControlBlockConfig {
	return e.cfg
}
