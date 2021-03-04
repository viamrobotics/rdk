package board

import (
	"fmt"
	"time"

	"github.com/erh/scheme"

	"go.viam.com/robotcore/utils"
)

func createDigitalInterrupt(cfg DigitalInterruptConfig) (DigitalInterrupt, error) {
	if cfg.Type == "" {
		cfg.Type = "basic"
	}

	var i DigitalInterrupt
	switch cfg.Type {
	case "basic":
		i = &BasicDigitalInterrupt{cfg: cfg}
	case "servo":
		i = &ServoDigitalInterrupt{cfg: cfg, ra: utils.NewRollingAverage(15)}
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

	callbacks []diCallback

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

func (i *BasicDigitalInterrupt) Tick() {
	i.count++

	for {
		got := false

		for idx, c := range i.callbacks {
			if i.count < c.threshold {
				continue
			}

			c.c <- i.count
			i.callbacks = append(i.callbacks[0:idx], i.callbacks[idx+1:]...)
			got = true
			break
		}
		if !got {
			break
		}
	}
}

func (i *BasicDigitalInterrupt) AddCallbackDelta(delta int64, c chan int64) {
	i.callbacks = append(i.callbacks, diCallback{i.count + delta, c})
}

func (i *BasicDigitalInterrupt) AddPostProcess(pp PostProcess) {
	i.pp = pp
}

// ----

type ServoDigitalInterrupt struct {
	cfg  DigitalInterruptConfig
	last int64
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

func (i *ServoDigitalInterrupt) Tick() {
	now := time.Now().UnixNano()
	diff := now - i.last
	i.last = now

	if diff > int64(10*time.Millisecond) {
		// this is time between signals, ignore
		return
	}

	i.ra.Add(int(diff / 1000))
}

func (i *ServoDigitalInterrupt) AddCallbackDelta(delta int64, c chan int64) {
	panic(fmt.Errorf("servos can't have callback deltas"))
}

func (i *ServoDigitalInterrupt) AddPostProcess(pp PostProcess) {
	i.pp = pp
}
