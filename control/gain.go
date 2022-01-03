package control

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
)

type gain struct {
	mu     sync.Mutex
	cfg    ControlBlockConfig
	y      []Signal
	gain   float64
	logger golog.Logger
}

func newGain(config ControlBlockConfig, logger golog.Logger) (ControlBlock, error) {
	g := &gain{cfg: config, logger: logger}
	if err := g.reset(); err != nil {
		return nil, err
	}
	return g, nil
}

func (b *gain) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	if len(x) != 1 {
		return b.y, false
	}
	b.y[0].SetSignalValueAt(0, 0.0)
	for _, s := range x {
		tx := s.GetSignalValueAt(0)
		b.y[0].SetSignalValueAt(0, tx*b.gain)
	}
	return b.y, true
}

func (b *gain) reset() error {
	if !b.cfg.Attribute.Has("gain") {
		return errors.Errorf("gain block %s doesn't have a gain field", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) != 1 {
		return errors.Errorf("invalid number of inputs for gain block %s expected 1 got %d", b.cfg.Name, len(b.cfg.DependsOn))
	}
	b.gain = b.cfg.Attribute.Float64("gain", 1.0)
	b.y = make([]Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, 1)
	return nil
}

func (b *gain) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset()
}

func (b *gain) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset()
}

func (b *gain) Output(ctx context.Context) []Signal {
	return b.y
}

func (b *gain) Config(ctx context.Context) ControlBlockConfig {
	return b.cfg
}
