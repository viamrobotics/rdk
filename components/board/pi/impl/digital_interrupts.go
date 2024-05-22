//go:build linux && (arm64 || arm) && !no_pigpio && !no_cgo

package piimpl

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

// DigitalInterruptConfig describes the configuration of digital interrupt for the board.
type DigitalInterruptConfig struct {
	Name string `json:"name"`
	Pin  string `json:"pin"`
	Type string `json:"type,omitempty"` // e.g. basic, servo
}

// Validate ensures all parts of the config are valid.
func (config *DigitalInterruptConfig) Validate(path string) error {
	if config.Name == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "name")
	}
	if config.Pin == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "pin")
	}
	return nil
}

// ServoRollingAverageWindow is how many entries to average over for
// servo ticks.
const ServoRollingAverageWindow = 10

// A ReconfigurableDigitalInterrupt is a simple reconfigurable digital interrupt that expects
// reconfiguration within the same type.
type ReconfigurableDigitalInterrupt interface {
	board.DigitalInterrupt
	Reconfigure(cfg DigitalInterruptConfig) error
}

// CreateDigitalInterrupt is a factory method for creating a specific DigitalInterrupt based
// on the given config. If no type is specified, a BasicDigitalInterrupt is returned.
func CreateDigitalInterrupt(cfg DigitalInterruptConfig) (ReconfigurableDigitalInterrupt, error) {
	if cfg.Type == "" {
		cfg.Type = "basic"
	}

	var i ReconfigurableDigitalInterrupt
	switch cfg.Type {
	case "basic":
		i = &BasicDigitalInterrupt{}
	case "servo":
		i = &ServoDigitalInterrupt{ra: utils.NewRollingAverage(ServoRollingAverageWindow)}
	default:
		panic(errors.Errorf("unknown interrupt type (%s)", cfg.Type))
	}

	if err := i.Reconfigure(cfg); err != nil {
		return nil, err
	}
	return i, nil
}

// A BasicDigitalInterrupt records how many ticks/interrupts happen and can
// report when they happen to interested callbacks.
type BasicDigitalInterrupt struct {
	count int64

	callbacks []chan board.Tick

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

// Tick records an interrupt and notifies any interested callbacks. See comment on
// the DigitalInterrupt interface for caveats.
func Tick(ctx context.Context, i *BasicDigitalInterrupt, high bool, nanoseconds uint64) error {
	if high {
		atomic.AddInt64(&i.count, 1)
	}

	i.mu.RLock()
	defer i.mu.RUnlock()
	for _, c := range i.callbacks {
		select {
		case <-ctx.Done():
			return errors.New("context cancelled")
		case c <- board.Tick{Name: i.cfg.Name, High: high, TimestampNanosec: nanoseconds}:
		}
	}
	return nil
}

// AddCallback adds a listener for interrupts.
func AddCallback(i *BasicDigitalInterrupt, c chan board.Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.callbacks = append(i.callbacks, c)
}

// RemoveCallback removes a listener for interrupts.
func RemoveCallback(i *BasicDigitalInterrupt, c chan board.Tick) {
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

// Name returns the name of the interrupt.
func (i *BasicDigitalInterrupt) Name() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg.Name
}

// Reconfigure reconfigures this digital interrupt.
func (i *BasicDigitalInterrupt) Reconfigure(conf DigitalInterruptConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.cfg = conf
	return nil
}

// A ServoDigitalInterrupt is an interrupt associated with a servo in order to
// track the amount of time that has passed between low signals (pulse width). Post processors
// make meaning of these widths.
type ServoDigitalInterrupt struct {
	last uint64
	ra   *utils.RollingAverage

	mu  sync.RWMutex
	cfg DigitalInterruptConfig
}

// Value will return the window averaged value followed by its post processed
// result.
func (i *ServoDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	v := int64(i.ra.Average())
	return v, nil
}

// ServoTick records the time between two successive low signals (pulse width). How it is
// interpreted is based off the consumer of Value.
func ServoTick(ctx context.Context, i *ServoDigitalInterrupt, high bool, now uint64) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	diff := now - i.last
	i.last = now

	if i.last == 0 {
		return nil
	}

	if high {
		// this is time between signals, ignore
		return nil
	}

	i.ra.Add(int(diff / 1000))
	return nil
}

// Name returns the name of the interrupt.
func (i *ServoDigitalInterrupt) Name() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.cfg.Name
}

// Reconfigure reconfigures this digital interrupt.
func (i *ServoDigitalInterrupt) Reconfigure(conf DigitalInterruptConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.cfg = conf
	return nil
}
