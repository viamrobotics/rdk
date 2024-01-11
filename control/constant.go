package control

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

type constant struct {
	mu       sync.Mutex
	cfg      BlockConfig
	y        []*Signal
	constant float64
	logger   logging.Logger
}

func newConstant(config BlockConfig, logger logging.Logger) (Block, error) {
	c := &constant{cfg: config, logger: logger}
	if err := c.reset(); err != nil {
		return nil, err
	}
	return c, nil
}

func (b *constant) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	return b.y, true
}

func (b *constant) reset() error {
	if !b.cfg.Attribute.Has("constant_val") {
		return errors.Errorf("constant block %s doesn't have a constant_val field", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) > 0 {
		return errors.Errorf("invalid number of inputs for constant block %s expected 0 got %d", b.cfg.Name, len(b.cfg.DependsOn))
	}
	b.constant = b.cfg.Attribute["constant_val"].(float64) // default 0
	b.y = make([]*Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, b.cfg.Type)
	b.y[0].SetSignalValueAt(0, b.constant)
	return nil
}

func (b *constant) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset()
}

func (b *constant) UpdateConfig(ctx context.Context, config BlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset()
}

func (b *constant) Output(ctx context.Context) []*Signal {
	return b.y
}

func (b *constant) Config(ctx context.Context) BlockConfig {
	return b.cfg
}
