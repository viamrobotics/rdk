package control

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"

	"go.viam.com/rdk/logging"
)

type encoderToRPM struct {
	mu                 sync.Mutex
	cfg                BlockConfig
	y                  []*Signal
	ticksPerRevolution int
	prevEncCount       int
	logger             logging.Logger
}

func newEncoderSpeed(config BlockConfig, logger logging.Logger) (Block, error) {
	e := &encoderToRPM{cfg: config, logger: logger}
	if err := e.reset(); err != nil {
		return nil, err
	}
	return e, nil
}

func (b *encoderToRPM) Next(ctx context.Context, x []*Signal, dt time.Duration) ([]*Signal, bool) {
	currEncCount := int(x[0].GetSignalValueAt(0))
	b.y[0].SetSignalValueAt(0, (float64(currEncCount-b.prevEncCount)/float64(b.ticksPerRevolution))*60.0/(dt.Seconds()))
	b.prevEncCount = currEncCount
	return b.y, true
}

func (b *encoderToRPM) reset() error {
	if !b.cfg.Attribute.Has("ticks_per_revolution") {
		return errors.Errorf("encoderToRPM block %s doesn't have a ticks_per_revolution field", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) != 1 {
		return errors.Errorf("invalid number of inputs for encoderToRPM block %s expected 1 got %d", b.cfg.Name, len(b.cfg.DependsOn))
	}
	b.ticksPerRevolution = b.cfg.Attribute["ticks_per_revolution"].(int) // default 0
	b.prevEncCount = 0
	b.y = make([]*Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, b.cfg.Type)
	return nil
}

func (b *encoderToRPM) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset()
}

func (b *encoderToRPM) UpdateConfig(ctx context.Context, config BlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset()
}

func (b *encoderToRPM) Output(ctx context.Context) []*Signal {
	return b.y
}

func (b *encoderToRPM) Config(ctx context.Context) BlockConfig {
	return b.cfg
}
