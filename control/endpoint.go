package control

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
	pb "go.viam.com/core/proto/api/v1"
)

type endpoint struct {
	mu  sync.Mutex
	ctr Controllable
	cfg ControlBlockConfig
	y   []Signal
}

func (e *endpoint) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) == 1 {
		power := x[0].signal[0]
		if e.ctr != nil {
			err := e.ctr.Go(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, float32(power))
			if err != nil {
				return []Signal{}, false
			}
		}
		return []Signal{}, false
	}
	if len(x) == 0 {
		if e.ctr != nil {
			pos, err := e.ctr.Position(ctx)
			if err != nil {
				return []Signal{}, false
			}
			e.y[0].signal[0] = pos
		}
		return e.y, true
	}
	return e.y, false
}

func (e *endpoint) reset(ctx context.Context) error {
	if !e.cfg.Attribute.Has("MotorName") {
		return errors.Errorf("endpoint %s should have a MotorName field", e.cfg.Name)
	}
	e.y = make([]Signal, 1)
	e.y[0] = makeSignal(e.cfg.Name, 1)
	return nil
}

func (e *endpoint) Configure(ctx context.Context, config ControlBlockConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = config
	return e.reset(ctx)
}
func (e *endpoint) Reset(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.reset(ctx)
}
func (e *endpoint) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cfg = config
	return e.reset(ctx)
}
func (e *endpoint) Output(ctx context.Context) []Signal {
	return e.y
}

func (e *endpoint) Config(ctx context.Context) ControlBlockConfig {
	return e.cfg
}
