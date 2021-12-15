package control

import (
	"context"
	"sync"
	"time"

	"github.com/go-errors/errors"
)

type encoderToRPM struct {
	mu                 sync.Mutex
	cfg                ControlBlockConfig
	y                  []Signal
	pulsesPerReolution int
	prevEncCount       int
}

func (b *encoderToRPM) Next(ctx context.Context, x []Signal, dt time.Duration) ([]Signal, bool) {
	currEncCount := int(x[0].signal[0])
	b.y[0].signal[0] = (float64(currEncCount-b.prevEncCount) / float64(b.pulsesPerReolution)) * 60.0 / (dt.Seconds())
	b.prevEncCount = currEncCount
	return b.y, true
}
func (b *encoderToRPM) reset(ctx context.Context) error {
	if !b.cfg.Attribute.Has("PulsesPerRevolution") {
		return errors.Errorf("encoderToRPM block %s doesn't have a PulsesPerRevolution field", b.cfg.Name)
	}
	if len(b.cfg.DependsOn) != 1 {
		return errors.Errorf("invalid number of inputs for encoderToRPM block %s expected 1 got %d", b.cfg.Name, len(b.cfg.DependsOn))
	}
	b.pulsesPerReolution = b.cfg.Attribute.Int("PulsesPerRevolution", 0)
	b.prevEncCount = 0
	b.y = make([]Signal, 1)
	b.y[0] = makeSignal(b.cfg.Name, 1)
	return nil
}

func (b *encoderToRPM) Configure(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset(ctx)
}
func (b *encoderToRPM) Reset(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reset(ctx)
}
func (b *encoderToRPM) UpdateConfig(ctx context.Context, config ControlBlockConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = config
	return b.reset(ctx)
}

func (b *encoderToRPM) Output(ctx context.Context) []Signal {
	return b.y
}

func (b *encoderToRPM) Config(ctx context.Context) ControlBlockConfig {
	return b.cfg
}
