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
	e.logger.CInfof(ctx, "z length %v", len(x))
	e.logger.CInfof(ctx, "controllable is %v", e.ctr)
	switch len(x) {
	case 1, 2:
		if e.ctr != nil {
			e.logger.CInfof(ctx, "setting state %v", x)
			err := e.ctr.SetState(ctx, x)
			if err != nil {
				return []*Signal{}, false
			}
		}
		return []*Signal{}, false
	case 0:
		if e.ctr != nil {
			e.logger.CInfo(ctx, "case 0")
			vals, err := e.ctr.State(ctx)
			if err != nil {
				return []*Signal{}, false
			}
			for idx, val := range vals {
				e.logger.CInfof(ctx, "length val %v.  e.y %v", len(vals), e.y)
				e.y[idx].SetSignalValueAt(0, val)
			}
		}
		return e.y, true
	default:
		return e.y, false
	}
}

func (e *endpoint) reset() error {
	_, motorOk := e.cfg.Attribute["motor_name"]
	if motorOk {
		e.logger.Info("making a signal of length 1")
		e.y = make([]*Signal, 1)
		e.y[0] = makeSignal(e.cfg.Name)
	}

	_, baseOk := e.cfg.Attribute["base_name"]
	if baseOk {
		e.logger.Info("making a signal of length 2")
		e.y = make([]*Signal, 2)
		e.y[0] = makeSignal(e.cfg.Name)
		e.y[1] = makeSignal(e.cfg.Name)
	}

	if !motorOk && !baseOk {
		return errors.Errorf("endpoint %s should have a motor_name field", e.cfg.Name)
	}

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
