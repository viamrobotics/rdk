package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// DigitalInterrupt is an injected digital interrupt.
type DigitalInterrupt struct {
	board.DigitalInterrupt
	ValueFunc            func(ctx context.Context, extra map[string]interface{}) (int64, error)
	valueCap             []interface{}
	TickFunc             func(ctx context.Context, high bool, nanoseconds uint64) error
	tickCap              []interface{}
	AddCallbackFunc      func(c chan board.Tick)
	AddPostProcessorFunc func(pp board.PostProcessor)
}

// Value calls the injected Value or the real version.
func (d *DigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	d.valueCap = []interface{}{ctx}
	if d.ValueFunc == nil {
		return d.DigitalInterrupt.Value(ctx, extra)
	}
	return d.ValueFunc(ctx, extra)
}

// ValueCap returns the last parameters received by Value, and then clears them.
func (d *DigitalInterrupt) ValueCap() []interface{} {
	if d == nil {
		return nil
	}
	defer func() { d.valueCap = nil }()
	return d.valueCap
}

// Tick calls the injected Tick or the real version.
func (d *DigitalInterrupt) Tick(ctx context.Context, high bool, nanoseconds uint64) error {
	d.tickCap = []interface{}{ctx, high, nanoseconds}
	if d.TickFunc == nil {
		return d.DigitalInterrupt.Tick(ctx, high, nanoseconds)
	}
	return d.TickFunc(ctx, high, nanoseconds)
}

// TickCap returns the last parameters received by Tick, and then clears them.
func (d *DigitalInterrupt) TickCap() []interface{} {
	if d == nil {
		return nil
	}
	defer func() { d.tickCap = nil }()
	return d.tickCap
}

// AddCallback calls the injected AddCallback or the real version.
func (d *DigitalInterrupt) AddCallback(c chan board.Tick) {
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
