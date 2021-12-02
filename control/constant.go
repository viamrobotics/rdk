package control

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
)

type constant struct {
	mu       sync.Mutex
	cfg      ControlBlockConfig
	y        []Signal
	constant float64
}

func (b *constant) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	b.y[0].signal[0] = b.constant
	return b.y, true
}
func (b *constant) reset(ctx context.Context) error {
	if !b.cfg.Attribute.Has("ConstantVal") {
		return errors.Errorf("constant block %s doesn't have a ConstantVal field", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) > 0 {
		return errors.Errorf("invalid number of inputs for constant block %s expected 0 got %d", b.cfg.Name, len(b.cfg.DependsOn))
	}
	b.constant = b.cfg.Attribute.Float64("ConstantVal", 0.0)
	b.y = make([]Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, 1)
	return nil
}

func (b *constant) Configure(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset(ctx)
}
func (b *constant) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset(ctx)
}
func (b *constant) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset(ctx)
}

func (b *constant) Output(ctx context.Context) []Signal {
	return b.y
}

func (b *constant) Config(ctx context.Context) ControlBlockConfig {
	return b.cfg
}
