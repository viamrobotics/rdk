package inject

import (
	"context"

	"go.viam.com/core/component/board"
)

// DigitalInterrupt is an injected digital interrupt.
type DigitalInterrupt struct {
	board.DigitalInterrupt
	ConfigFunc           func(ctx context.Context) (board.DigitalInterruptConfig, error)
	ValueFunc            func(ctx context.Context) (int64, error)
	TickFunc             func(ctx context.Context, high bool, nanos uint64) error
	AddCallbackFunc      func(c chan bool)
	AddPostProcessorFunc func(pp board.PostProcessor)
}

// Config calls the injected Config or the real version.
func (d *DigitalInterrupt) Config(ctx context.Context) (board.DigitalInterruptConfig, error) {
	if d.ConfigFunc == nil {
		return d.DigitalInterrupt.Config(ctx)
	}
	return d.ConfigFunc(ctx)
}

// Value calls the injected Value or the real version.
func (d *DigitalInterrupt) Value(ctx context.Context) (int64, error) {
	if d.ValueFunc == nil {
		return d.DigitalInterrupt.Value(ctx)
	}
	return d.ValueFunc(ctx)
}

// Tick calls the injected Tick or the real version.
func (d *DigitalInterrupt) Tick(ctx context.Context, high bool, nanos uint64) error {
	if d.TickFunc == nil {
		return d.DigitalInterrupt.Tick(ctx, high, nanos)
	}
	return d.TickFunc(ctx, high, nanos)
}

// AddCallback calls the injected AddCallback or the real version.
func (d *DigitalInterrupt) AddCallback(c chan bool) {
	if d.AddCallbackFunc == nil {
		d.DigitalInterrupt.AddCallback(c)
		return
	}
	d.AddCallbackFunc(c)
}

// AddPostProcessor calls the injected AddPostProcessor or the real version.
func (d *DigitalInterrupt) AddPostProcessor(pp board.PostProcessor) {
	if d.AddPostProcessorFunc == nil {
		d.DigitalInterrupt.AddPostProcessor(pp)
		return
	}
	d.AddPostProcessorFunc(pp)
}
