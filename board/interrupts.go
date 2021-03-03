package board

import (
	"fmt"
	"time"
)

func createDigitalInterrupt(cfg DigitalInterruptConfig) DigitalInterrupt {
	if cfg.Type == "" {
		cfg.Type = "basic"
	}

	switch cfg.Type {
	case "basic":
		return &BasicDigitalInterrupt{cfg: cfg}
	case "servo":
		return &ServoDigitalInterrupt{cfg: cfg}
	default:
		panic(fmt.Errorf("unknown interrupt type (%s)", cfg.Type))
	}
}

type BasicDigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	count int64

	callbacks []diCallback
}

func (i *BasicDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

func (i *BasicDigitalInterrupt) Value() int64 {
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

// ----

type ServoDigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	last  int64
	value int64
}

func (i *ServoDigitalInterrupt) Config() DigitalInterruptConfig {
	return i.cfg
}

func (i *ServoDigitalInterrupt) Value() int64 {
	return i.value
}

func (i *ServoDigitalInterrupt) Tick() {
	now := time.Now().UnixNano()
	diff := now - i.last
	i.last = now

	if diff > int64(10*time.Millisecond) {
		// this is time between signals, ignore
		return
	}

	i.value = diff / 1000
}

func (i *ServoDigitalInterrupt) AddCallbackDelta(delta int64, c chan int64) {
	panic(fmt.Errorf("servos can't have callback deltas"))
}
