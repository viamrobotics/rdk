package control

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

type endpoint struct {
	mu     sync.Mutex
	ctr    Controllable
	cfg    BlockConfig
	y      []*Signal
	logger logging.Logger
}

func newEndpoint(config BlockConfig, logger logging.Logger, ctr Controllable) (Block, error) {
	e := &endpoint{cfg: config, logger: logger, ctr: ctr}
	if err := e.reset(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *endpoint) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	switch len(x) {
	case 1:
		power := x[0].GetSignalValueAt(0)
		if e.ctr != nil {
			err := e.ctr.SetState(ctx, power)
			if err != nil {
				return []*Signal{}, false
			}
		}
		return []*Signal{}, false
	case 0:
		if e.ctr != nil {
			pos, err := e.ctr.State(ctx)
			if err != nil {
				return []*Signal{}, false
			}
			e.y[0].SetSignalValueAt(0, pos)
		}
		return e.y, true
	default:
		return e.y, false
	}
}

func (e *endpoint) reset() error {
	if !e.cfg.Attribute.Has("motor_name") {
		return errors.Errorf("endpoint %s should have a motor_name field", e.cfg.Name)
	}
	e.y = make([]*Signal, 1)
	e.y[0] = makeSignal(e.cfg.Name)
	return nil
}

func (e *endpoint) Reset(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reset()
}

func (e *endpoint) UpdateConfig(ctx context.Context, config BlockConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = config
	return e.reset()
}

func (e *endpoint) Output(ctx context.Context) []*Signal {
	return e.y
}

func (e *endpoint) Config(ctx context.Context) BlockConfig {
	return e.cfg
}
