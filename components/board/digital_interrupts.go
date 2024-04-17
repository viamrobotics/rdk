package board

import (
	"context"
)

// Tick represents a signal received by an interrupt pin. This signal is communicated
// via registered channel to the various drivers. Depending on board implementation there may be a
// wraparound in timestamp values past 4294967295000 nanoseconds (~72 minutes) if the value
// was originally in microseconds as a 32-bit integer. The timestamp in nanoseconds of the
// tick SHOULD ONLY BE USED FOR CALCULATING THE TIME ELAPSED BETWEEN CONSECUTIVE TICKS AND NOT
// AS AN ABSOLUTE TIMESTAMP.
type Tick struct {
	Name             string
	High             bool
	TimestampNanosec uint64
}

// A DigitalInterrupt represents a configured interrupt on the board that
// when interrupted, calls the added callbacks.
type DigitalInterrupt interface {
	// Value returns the current value of the interrupt which is
	// based on the type of interrupt.
	Value(ctx context.Context, extra map[string]interface{}) (int64, error)

	// Tick is to be called either manually if the interrupt is a proxy to some real
	// hardware interrupt or for tests.
	// nanoseconds is from an arbitrary point in time, but always increasing and always needs
	// to be accurate.
	Tick(ctx context.Context, high bool, nanoseconds uint64) error

	// AddCallback adds a callback to be sent a low/high value to when a tick
	// happens.
	AddCallback(c chan Tick)

	Close(ctx context.Context) error
}
