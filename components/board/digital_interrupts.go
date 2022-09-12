package board

import (
	"context"
	"sync/atomic"

	"github.com/erh/scheme"
	"github.com/pkg/errors"

	"go.viam.com/rdk/utils"
)

// ServoRollingAverageWindow is how many entries to average over for
// servo ticks.
const ServoRollingAverageWindow = 10

// A DigitalInterrupt represents a configured interrupt on the board that
// when interrupted, calls the added callbacks. Post processors can also
// be added to modify what Value ultimately returns.
type DigitalInterrupt interface {
	// Value returns the current value of the interrupt which is
	// based on the type of interrupt.
	Value(ctx context.Context, extra map[string]interface{}) (int64, error)

	// Tick is to be called either manually if the interrupt is a proxy to some real
	// hardware interrupt or for tests.
	// nanos is from an arbitrary point in time, but always increasing and always needs
	// to be accurate. Using time.Now().UnixNano() would be acceptable, but is
	// not required.
	Tick(ctx context.Context, high bool, nanos uint64) error

	// AddCallback adds a callback to be sent a low/high value to when a tick
	// happens.
	// Note(erd): not all interrupts can have callbacks so this should probably be a
	// separate interface.
	AddCallback(c chan bool)

	// AddPostProcessor adds a post processor that should be used to modify
	// what is returned by Value.
	AddPostProcessor(pp PostProcessor)
}

// CreateDigitalInterrupt is a factory method for creating a specific DigitalInterrupt based
// on the given config. If no type is specified, a BasicDigitalInterrupt is returned.
func CreateDigitalInterrupt(cfg DigitalInterruptConfig) (DigitalInterrupt, error) {
	if cfg.Type == "" {
		cfg.Type = "basic"
	}

	var i DigitalInterrupt
	switch cfg.Type {
	case "basic":
		iActual := &BasicDigitalInterrupt{cfg: cfg}
		i = iActual
	case "servo":
		iActual := &ServoDigitalInterrupt{cfg: cfg, ra: utils.NewRollingAverage(ServoRollingAverageWindow)}
		i = iActual
	default:
		panic(errors.Errorf("unknown interrupt type (%s)", cfg.Type))
	}

	if cfg.Formula != "" {
		x, err := scheme.Parse(cfg.Formula)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't parse formula for %s", cfg.Name)
		}

		testScope := scheme.Scope{}
		num := 1.0
		testScope["raw"] = &scheme.Value{Float: &num}
		_, err = scheme.Eval(x, testScope)
		if err != nil {
			return nil, errors.Wrapf(err, "test exec failed for %s", cfg.Name)
		}

		i.AddPostProcessor(func(raw int64) int64 {
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
		})
	}

	return i, nil
}

// A BasicDigitalInterrupt records how many ticks/interrupts happen and can
// report when they happen to interested callbacks.
type BasicDigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	count int64

	callbacks []chan bool

	pp PostProcessor
}

// Config returns the config used to create this interrupt.
func (i *BasicDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	return i.cfg, nil
}

// Value returns the amount of ticks that have occurred.
func (i *BasicDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
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

// Tick records an interrupt and notifies any interested callbacks.
func (i *BasicDigitalInterrupt) Tick(ctx context.Context, high bool, not uint64) error {
	if high {
		atomic.AddInt64(&i.count, 1)
	}

	for _, c := range i.callbacks {
		c <- high
	}

	return nil
}

// AddCallback adds a listener for interrupts.
func (i *BasicDigitalInterrupt) AddCallback(c chan bool) {
	i.callbacks = append(i.callbacks, c)
}

// AddPostProcessor sets the post processor that will modify the value that
// Value returns.
func (i *BasicDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	i.pp = pp
}

// A ServoDigitalInterrupt is an interrupt associated with a servo in order to
// track the amount of time that has passed between low signals (pulse width). Post processors
// make meaning of these widths.
type ServoDigitalInterrupt struct {
	cfg  DigitalInterruptConfig
	last uint64
	ra   *utils.RollingAverage
	pp   PostProcessor
}

// Config returns the config the interrupt was created with.
func (i *ServoDigitalInterrupt) Config(ctx context.Context) (DigitalInterruptConfig, error) {
	return i.cfg, nil
}

// Value will return the window averaged value followed by its post processed
// result.
func (i *ServoDigitalInterrupt) Value(ctx context.Context, extra map[string]interface{}) (int64, error) {
	v := int64(i.ra.Average())
	if i.pp != nil {
		return i.pp(v), nil
	}

	return v, nil
}

// Tick records the time between two successive low signals (pulse width). How it is
// interpreted is based off the consumer of Value.
func (i *ServoDigitalInterrupt) Tick(ctx context.Context, high bool, now uint64) error {
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
func (i *ServoDigitalInterrupt) AddCallback(c chan bool) {
	panic("servos can't have callback")
}

// AddPostProcessor sets the post processor that will modify the value that
// Value returns.
func (i *ServoDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	i.pp = pp
}
