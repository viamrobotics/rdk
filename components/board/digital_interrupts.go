package board

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/erh/scheme"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
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
	High             bool
	TimestampNanosec uint64
}

// A DigitalInterrupt represents a configured interrupt on the board that
// when interrupted, calls the added callbacks. Post processors can also
// be added to modify what Value ultimately returns.
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

	// AddPostProcessor adds a post processor that should be used to modify
	// what is returned by Value.
	AddPostProcessor(pp PostProcessor)

	// RemoveCallback removes a listener for interrupts
	RemoveCallback(c chan Tick)

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
func CreateDigitalInterrupt(cfg DigitalInterruptConfig) (DigitalInterrupt, error) {
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

	callbacks []chan Tick

	mu  sync.RWMutex
	cfg DigitalInterruptConfig
	pp  PostProcessor
}

// Config returns the config used to create this interrupt.
func (i *BasicDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.cfg, nil
}

// Value returns the amount of ticks that have occurred.
func (i *BasicDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	count := atomic.LoadInt64(&i.count)
	if i.pp != nil {
		return i.pp(count), nil
	}
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
		case c <- Tick{High: high, TimestampNanosec: nanoseconds}:
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

// AddPostProcessor sets the post processor that will modify the value that
// Value returns.
func (i *BasicDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.pp = pp
}

// Close does nothing.
func (i *BasicDigitalInterrupt) Close(ctx context.Context) error {
	return nil
}

func processFormula(oldFormula, newFormula, name string) (func(raw int64) int64, bool, error) {
	if oldFormula == newFormula {
		return nil, false, nil
	}
	x, err := scheme.Parse(newFormula)
	if err != nil {
		return nil, false, errors.Wrapf(err, "couldn't parse formula for %s", name)
	}

	testScope := scheme.Scope{}
	num := 1.0
	testScope["raw"] = &scheme.Value{Float: &num}
	_, err = scheme.Eval(x, testScope)
	if err != nil {
		return nil, false, errors.Wrapf(err, "test exec failed for %s", name)
	}

	return func(raw int64) int64 {
		scope := scheme.Scope{}
		rr := float64(raw) // TODO(erh): fix
		scope["raw"] = &scheme.Value{Float: &rr}
		res, err := scheme.Eval(x, scope)
		if err != nil {
			panic(err)
		}
		f, err := res.ToFloat()
		if err != nil {
			panic(err)
		}
		return int64(f)
	}, true, nil
}

// Reconfigure reconfigures this digital interrupt with a new formula.
func (i *BasicDigitalInterrupt) Reconfigure(conf DigitalInterruptConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	newFormula, isNew, err := processFormula(i.cfg.Formula, conf.Formula, conf.Name)
	if err != nil {
		return err
	}
	if !isNew {
		return nil
	}
	i.pp = newFormula
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
	pp  PostProcessor
}

// Config returns the config the interrupt was created with.
func (i *ServoDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.cfg, nil
}

// Value will return the window averaged value followed by its post processed
// result.
func (i *ServoDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	v := int64(i.ra.Average())
	if i.pp != nil {
		return i.pp(v), nil
	}

	return v, nil
}

// Tick records the time between two successive low signals (pulse width). How it is
// interpreted is based off the consumer of Value.
func (i *ServoDigitalInterrupt) Tick(ctx context.Context, high bool, now uint64) error {
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

// AddCallback currently panics.
func (i *ServoDigitalInterrupt) AddCallback(c chan Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	panic("servos can't have callback")
}

// RemoveCallback currently panics.
func (i *ServoDigitalInterrupt) RemoveCallback(c chan Tick) {
	i.mu.Lock()
	defer i.mu.Unlock()
	panic("servos can't have callback")
}

// AddPostProcessor sets the post processor that will modify the value that
// Value returns.
func (i *ServoDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.pp = pp
}

// Reconfigure reconfigures this digital interrupt with a new formula.
func (i *ServoDigitalInterrupt) Reconfigure(conf DigitalInterruptConfig) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	newFormula, isNew, err := processFormula(i.cfg.Formula, conf.Formula, conf.Name)
	if err != nil {
		return err
	}
	if !isNew {
		return nil
	}
	i.pp = newFormula
	i.cfg = conf
	return nil
}

// Close does nothing.
func (i *ServoDigitalInterrupt) Close(ctx context.Context) error {
	return nil
}
