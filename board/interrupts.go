package board

import (
	"fmt"

	"github.com/erh/scheme"

	"go.viam.com/robotcore/utils"
)

const (
	ServoRollingAverageWidndow = 10
)

func CreateDigitalInterrupt(cfg DigitalInterruptConfig) (DigitalInterrupt, error) {
	if cfg.Type == "" {
		cfg.Type = "basic"
	}

	var i DigitalInterrupt
	switch cfg.Type {
	case "basic":
		i = &BasicDigitalInterrupt{cfg: cfg}
	case "servo":
		i = &ServoDigitalInterrupt{cfg: cfg, ra: utils.NewRollingAverage(ServoRollingAverageWidndow)}
	default:
		panic(fmt.Errorf("unknown interrupt type (%s)", cfg.Type))
	}

	if cfg.Formula != "" {
		x, err := scheme.Parse(cfg.Formula)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse formula for %s %s", cfg.Name, err)
		}

		testScope := scheme.Scope{}
		num := 1.0
		testScope["raw"] = &scheme.Value{Float: &num}
		_, err = scheme.Eval(x, testScope)
		if err != nil {
			return nil, fmt.Errorf("test exec failed for %s %s", cfg.Name, err)
		}

		i.AddPostProcess(func(raw int64) int64 {
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

type BasicDigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	count int64

	callbacks []chan bool

	pp PostProcess
}

func (i *BasicDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

func (i *BasicDigitalInterrupt) Value() int64 {
	if i.pp != nil {
		return i.pp(i.count)
	}
	return i.count
}

// really just for testing
func (i *BasicDigitalInterrupt) ticks(num int, now uint64) {
	for x := 0; x < num; x++ {
		i.Tick(true, now+uint64(x))
	}
}

func (i *BasicDigitalInterrupt) Tick(high bool, not uint64) {
	if high {
		i.count++
	}

	for _, c := range i.callbacks {
		c <- high
	}
}

func (i *BasicDigitalInterrupt) AddCallback(c chan bool) {
	i.callbacks = append(i.callbacks, c)
}

func (i *BasicDigitalInterrupt) AddPostProcess(pp PostProcess) {
	i.pp = pp
}

// ----

type ServoDigitalInterrupt struct {
	cfg  DigitalInterruptConfig
	last uint64
	ra   *utils.RollingAverage
	pp   PostProcess
}

func (i *ServoDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

func (i *ServoDigitalInterrupt) Value() int64 {
	v := int64(i.ra.Average())
	if i.pp != nil {
		return i.pp(v)
	}

	return v
}

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
	panic(fmt.Errorf("servos can't have callback "))
}

func (i *ServoDigitalInterrupt) AddPostProcess(pp PostProcess) {
	i.pp = pp
}
