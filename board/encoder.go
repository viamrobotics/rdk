package board

import (
	"context"
	"sync"
	"sync/atomic"

	"go.viam.com/utils"
)

// Encoder keeps track of a motor position
type Encoder interface {
	// Position returns the current position in terms of ticks
	Position(ctx context.Context) (int64, error)

	// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
	// to be its new zero position
	ResetZeroPosition(ctx context.Context, offset int64) error

	// Start starts a background thread to run the encoder, if there is none needed this is a no-op
	Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func())
}

// ---------

// HallEncoder keeps track of a motor position using a rotary hall encoder
type HallEncoder struct {
	a, b     DigitalInterrupt
	position int64
}

// NewHallEncoder creates a new HallEncoder
func NewHallEncoder(a, b DigitalInterrupt) *HallEncoder {
	return &HallEncoder{a, b, 0}
}

// Start starts the HallEncoder background thread
func (e *HallEncoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func()) {
	/**
	  a rotary encoder looks like

	  picture from https://github.com/joan2937/pigpio/blob/master/EXAMPLES/C/ROTARY_ENCODER/rotary_encoder.c
	    1   2     3    4    1    2    3    4     1

	            +---------+         +---------+      0
	            |         |         |         |
	  A         |         |         |         |
	            |         |         |         |
	  +---------+         +---------+         +----- 1

	      +---------+         +---------+            0
	      |         |         |         |
	  B   |         |         |         |
	      |         |         |         |
	  ----+         +---------+         +---------+  1

	*/

	chanA := make(chan bool)
	chanB := make(chan bool)

	e.a.AddCallback(chanA)
	e.b.AddCallback(chanB)

	activeBackgroundWorkers.Add(1)

	utils.ManagedGo(func() {
		onStart()
		aLevelOnce := false
		bLevelOnce := false
		aLevel := true
		bLevel := true

		lastWasA := true
		lastLevel := true

		for {

			select {
			case <-cancelCtx.Done():
				return
			default:
			}

			var level bool
			var isA bool

			select {
			case <-cancelCtx.Done():
				return
			case level = <-chanA:
				aLevelOnce = true
				isA = true
				aLevel = level
			case level = <-chanB:
				bLevelOnce = true
				isA = false
				bLevel = level
			}

			if !(aLevelOnce && bLevelOnce) {
				// we need two physical ticks to make any state determination
				continue
			}

			if isA == lastWasA && level == lastLevel {
				// this means we got the exact same message multiple times
				// this is probably some sort of hardware issue, so we ignore
				continue
			}
			lastWasA = isA
			lastLevel = level

			if !aLevel && !bLevel { // state 1
				if lastWasA {
					e.inc()
				} else {
					e.dec()
				}
			} else if !aLevel && bLevel { // state 2
				if lastWasA {
					e.dec()
				} else {
					e.inc()
				}
			} else if aLevel && bLevel { // state 3
				if lastWasA {
					e.inc()
				} else {
					e.dec()
				}
			} else if aLevel && !bLevel { // state 4
				if lastWasA {
					e.dec()
				} else {
					e.inc()
				}
			}

		}
	}, activeBackgroundWorkers.Done)
}

// Position returns the current position
func (e *HallEncoder) Position(ctx context.Context) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position
func (e *HallEncoder) ResetZeroPosition(ctx context.Context, offset int64) error {
	atomic.StoreInt64(&e.position, offset)
	return nil
}

// RawPosition returns the raw position of the encoder.
func (e *HallEncoder) RawPosition() int64 {
	return atomic.LoadInt64(&e.position)
}

func (e *HallEncoder) inc() {
	atomic.AddInt64(&e.position, 1)
}

func (e *HallEncoder) dec() {
	atomic.AddInt64(&e.position, -1)
}

// ---------

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off
type DirectionAware interface {
	DirectionMoving() int64
}

// NewSingleEncoder creates a new SingleEncoder (da begins as nil)
func NewSingleEncoder(i DigitalInterrupt, da DirectionAware) *SingleEncoder {
	return &SingleEncoder{i: i, m: da}
}

// AttachDirectionalAwareness to pre-created encoder
func (e *SingleEncoder) AttachDirectionalAwareness(da DirectionAware) {
	e.m = da
}

// SingleEncoder is a single interrupt based encoder.
type SingleEncoder struct {
	i        DigitalInterrupt
	position int64
	m        DirectionAware
}

// Start starts up the encoder.
func (e *SingleEncoder) Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func()) {
	encoderChannel := make(chan bool)
	e.i.AddCallback(encoderChannel)
	activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		onStart()
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
			}

			select {
			case <-cancelCtx.Done():
				return
			case <-encoderChannel:
			}

			dir := e.m.DirectionMoving()
			if dir == 1 || dir == -1 {
				atomic.AddInt64(&e.position, dir)
			}
		}
	}, activeBackgroundWorkers.Done)
}

// Position returns the current position
func (e *SingleEncoder) Position(ctx context.Context) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position
func (e *SingleEncoder) ResetZeroPosition(ctx context.Context, offset int64) error {
	atomic.StoreInt64(&e.position, offset)
	return nil
}
