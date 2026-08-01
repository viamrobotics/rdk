package control

import (
	"context"
	"math"
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
	b.mu.Lock()
	defer b.mu.Unlock()
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
	// Block configs are decoded from JSON, so every number arrives as a float64 and a plain .(int)
	// assertion would panic on any real config.
	switch ticks := b.cfg.Attribute["ticks_per_revolution"].(type) {
	case int:
		b.ticksPerRevolution = ticks
	case float64:
		if ticks != math.Trunc(ticks) {
			return errors.Errorf("encoderToRPM block %s ticks_per_revolution must be a whole number, got %v", b.cfg.Name, ticks)
		}
		b.ticksPerRevolution = int(ticks)
	default:
		return errors.Errorf("encoderToRPM block %s ticks_per_revolution must be a number, got %T",
			b.cfg.Name, b.cfg.Attribute["ticks_per_revolution"])
	}
	// Guarding here rather than in Next keeps the divide-by-zero out of the control loop entirely.
	if b.ticksPerRevolution == 0 {
		return errors.Errorf("encoderToRPM block %s ticks_per_revolution must be nonzero", b.cfg.Name)
	}
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
