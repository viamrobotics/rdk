package board

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"
)

// ServoRollingAverageWindow is how many entries to average over for
// servo ticks.
const ServoRollingAverageWindow = 10

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
	AddCallback(ch chan Tick)

	// RemoveCallback removes a callback.
	RemoveCallback(ch chan Tick)

	Close(ctx context.Context) error
}

// A ReconfigurableDigitalInterrupt is a simple reconfigurable digital interrupt that expects
// reconfiguration within the same type.
type ReconfigurableDigitalInterrupt interface {
	DigitalInterrupt
	Reconfigure(cfg DigitalInterruptConfig) error
}

// CreateDigitalInterrupt is a factory method for creating a specific DigitalInterrupt based
// on the given config. If no type is specified, a BasicDigitalInterrupt is returned.
func CreateDigitalInterrupt(cfg DigitalInterruptConfig) (ReconfigurableDigitalInterrupt, error) {
	i := &BasicDigitalInterrupt{}

	if err := i.Reconfigure(cfg); err != nil {
		return nil, err
	}
	return i, nil
}

// A BasicDigitalInterrupt records how many ticks/interrupts happen and can
// report when they happen to interested callbacks.
type BasicDigitalInterrupt struct {
	count int64

	callbacks []chan Tick

	mu  sync.RWMutex
	cfg DigitalInterruptConfig
}

// Value returns the amount of ticks that have occurred.
func (i *BasicDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	count := atomic.LoadInt64(&i.count)
	return count, nil
}

// Ticks is really just for testing.
func (i *BasicDigitalInterrupt) Ticks(ctx context.Context, num int, now uint64) error {
	for x := 0; x < num; x++ {
		if err := i.Tick(ctx, true, now+uint64(x)); err != nil {
			return err
		}
	}
	return nil
}

// Tick records an interrupt and notifies any interested callbacks. See comment on
// the DigitalInterrupt interface for caveats.
func (i *BasicDigitalInterrupt) Tick(ctx context.Context, high bool, nanoseconds uint64) error {
	if high {
		atomic.AddInt64(&i.count, 1)
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, c := range i.callbacks {
		select {
		case <-ctx.Done():
			return errors.New("context cancelled")
		case c <- Tick{Name: i.cfg.Name, High: high, TimestampNanosec: nanoseconds}:
		}
	}
	return nil
}

// AddCallback adds a listener for interrupts.
func (i *BasicDigitalInterrupt) AddCallback(c chan Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.callbacks = append(i.callbacks, c)
}

// RemoveCallback removes a listener for interrupts.
func (i *BasicDigitalInterrupt) RemoveCallback(c chan Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for id := range i.callbacks {
		if i.callbacks[id] == c {
			// To remove this item, we replace it with the last item in the list, then truncate the
			// list by 1.
			i.callbacks[id] = i.callbacks[len(i.callbacks)-1]
			i.callbacks = i.callbacks[:len(i.callbacks)-1]
			break
		}
	}
}

// Close does nothing.
func (i *BasicDigitalInterrupt) Close(ctx context.Context) error {
	return nil
}

// Reconfigure reconfigures this digital interrupt with a new formula.
func (i *BasicDigitalInterrupt) Reconfigure(conf DigitalInterruptConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cfg = conf
	return nil
}
