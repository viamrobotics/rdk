package board

import (
	"fmt"
	"sync/atomic"

	"github.com/erh/scheme"

	"go.viam.com/robotcore/utils"
)

// ServoRollingAverageWindow is how many entries to average over for
// servo ticks.
const ServoRollingAverageWindow = 10

// A DigitalInterrupt represents a configured interrupt on the board that
// when interrupted, calls the added callbacks. Post processors can also
// be added to modify what Value ultimately returns.
type DigitalInterrupt interface {

	// Config returns the config the interrupt was created with.
	Config() DigitalInterruptConfig

	// Value returns the current value of the interrupt which is
	// based on the type of interrupt.
	Value() int64

	// Tick is to be called either manually if the interrupt is a proxy to some real
	// hardware interrupt or for tests.
	// nanos is from an arbitrary point in time, but always increasing and always needs
	// to be accurate. Using time.Now().UnixNano() would be acceptable, but is
	// not required.
	Tick(high bool, nanos uint64)

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
		i = &BasicDigitalInterrupt{cfg: cfg}
	case "servo":
		i = &ServoDigitalInterrupt{cfg: cfg, ra: utils.NewRollingAverage(ServoRollingAverageWindow)}
	default:
		panic(fmt.Errorf("unknown interrupt type (%s)", cfg.Type))
	}

	if cfg.Formula != "" {
		x, err := scheme.Parse(cfg.Formula)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse formula for %s %w", cfg.Name, err)
		}

		testScope := scheme.Scope{}
		num := 1.0
		testScope["raw"] = &scheme.Value{Float: &num}
		_, err = scheme.Eval(x, testScope)
		if err != nil {
			return nil, fmt.Errorf("test exec failed for %s %w", cfg.Name, err)
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

func (i *BasicDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

func (i *BasicDigitalInterrupt) Value() int64 {
	count := atomic.LoadInt64(&i.count)
	if i.pp != nil {
		return i.pp(count)
	}
	return count
}

// really just for testing
func (i *BasicDigitalInterrupt) ticks(num int, now uint64) {
	for x := 0; x < num; x++ {
		i.Tick(true, now+uint64(x))
	}
}

func (i *BasicDigitalInterrupt) Tick(high bool, not uint64) {
	if high {
		atomic.AddInt64(&i.count, 1)
	}

	for _, c := range i.callbacks {
		c <- high
	}
}

func (i *BasicDigitalInterrupt) AddCallback(c chan bool) {
	i.callbacks = append(i.callbacks, c)
}

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

func (i *ServoDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

// Value will return the window averaged value followed by its post processed
// result.
func (i *ServoDigitalInterrupt) Value() int64 {
	v := int64(i.ra.Average())
	if i.pp != nil {
		return i.pp(v)
	}

	return v
}

// Tick records the time between two successive low signals (pulse width). How it is
// interpreted is based off the consumer of Value.
func (i *ServoDigitalInterrupt) Tick(high bool, now uint64) {
	lastValid := i.last != 0

	diff := now - i.last
	i.last = now

	if !lastValid {
		return
	}

	if high {
		// this is time between signals, ignore
		return
	}

	i.ra.Add(int(diff / 1000))
}

func (i *ServoDigitalInterrupt) AddCallback(c chan bool) {
	panic("servos can't have callback")
}

func (i *ServoDigitalInterrupt) AddPostProcessor(pp PostProcessor) {
	i.pp = pp
}
