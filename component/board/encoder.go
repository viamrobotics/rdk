package board

import (
	"context"
	"sync"
	"sync/atomic"

	"go.viam.com/utils"
)

// Encoder keeps track of a motor position.
type Encoder interface {
	// GetPosition returns the current position in terms of ticks
	GetPosition(ctx context.Context, extra map[string]interface{}) (int64, error)

	// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
	// to be its new zero position
	ResetZeroPosition(ctx context.Context, offset int64, extra map[string]interface{}) error

	// Start starts a background thread to run the encoder, if there is none needed this is a no-op
	Start(cancelCtx context.Context, activeBackgroundWorkers *sync.WaitGroup, onStart func())
}

// ---------

// HallEncoder keeps track of a motor position using a rotary hall encoder.
type HallEncoder struct {
	a, b     DigitalInterrupt
	position int64
	pRaw     int64
	pState   int64
}

// NewHallEncoder creates a new HallEncoder.
func NewHallEncoder(a, b DigitalInterrupt) *HallEncoder {
	return &HallEncoder{a: a, b: b, position: 0, pRaw: 0, pState: 0}
}

// Start starts the HallEncoder background thread.
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

	// State Transition Table
	//     +---------------+----+----+----+----+
	//     | pState/nState | 00 | 01 | 10 | 11 |
	//     +---------------+----+----+----+----+
	//     |       00      | 0  | -1 | +1 | x  |
	//     +---------------+----+----+----+----+
	//     |       01      | +1 | 0  | x  | -1 |
	//     +---------------+----+----+----+----+
	//     |       10      | -1 | x  | 0  | +1 |
	//     +---------------+----+----+----+----+
	//     |       11      | x  | +1 | -1 | 0  |
	//     +---------------+----+----+----+----+
	// 0 -> same state
	// x -> impossible state

	chanA := make(chan bool)
	chanB := make(chan bool)

	e.a.AddCallback(chanA)
	e.b.AddCallback(chanB)

	aLevel, err := e.a.Value(cancelCtx)
	if err != nil {
		utils.Logger.Errorw("error reading a level", "error", err)
	}
	bLevel, err := e.b.Value(cancelCtx)
	if err != nil {
		utils.Logger.Errorw("error reading b level", "error", err)
	}
	e.pState = aLevel | (bLevel << 1)

	activeBackgroundWorkers.Add(1)

	utils.ManagedGo(func() {
		onStart()
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
			}

			var level bool

			select {
			case <-cancelCtx.Done():
				return
			case level = <-chanA:
				aLevel = 0
				if level {
					aLevel = 1
				}
			case level = <-chanB:
				bLevel = 0
				if level {
					bLevel = 1
				}
			}
			nState := aLevel | (bLevel << 1)
			if e.pState == nState {
				continue
			}
			switch (e.pState << 2) | nState {
			case 0b0001:
				fallthrough
			case 0b0111:
				fallthrough
			case 0b1000:
				fallthrough
			case 0b1110:
				e.dec()
				atomic.StoreInt64(&e.position, atomic.LoadInt64(&e.pRaw)>>1)
				e.pState = nState
			case 0b0010:
				fallthrough
			case 0b0100:
				fallthrough
			case 0b1011:
				fallthrough
			case 0b1101:
				e.inc()
				atomic.StoreInt64(&e.position, atomic.LoadInt64(&e.pRaw)>>1)
				e.pState = nState
			}
		}
	}, activeBackgroundWorkers.Done)
}

// GetPosition returns the current position.
func (e *HallEncoder) GetPosition(ctx context.Context, extra map[string]interface{}) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *HallEncoder) ResetZeroPosition(ctx context.Context, offset int64, extra map[string]interface{}) error {
	atomic.StoreInt64(&e.position, offset)
	atomic.StoreInt64(&e.pRaw, (offset<<1)|atomic.LoadInt64(&e.pRaw)&0x1)
	return nil
}

// RawPosition returns the raw position of the encoder.
func (e *HallEncoder) RawPosition() int64 {
	return atomic.LoadInt64(&e.pRaw)
}

func (e *HallEncoder) inc() {
	atomic.AddInt64(&e.pRaw, 1)
}

func (e *HallEncoder) dec() {
	atomic.AddInt64(&e.pRaw, -1)
}

// ---------

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// NewSingleEncoder creates a new SingleEncoder (da begins as nil).
func NewSingleEncoder(i DigitalInterrupt, da DirectionAware) *SingleEncoder {
	return &SingleEncoder{i: i, m: da}
}

// AttachDirectionalAwareness to pre-created encoder.
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

// GetPosition returns the current position.
func (e *SingleEncoder) GetPosition(ctx context.Context, extra map[string]interface{}) (int64, error) {
	return atomic.LoadInt64(&e.position), nil
}

// ResetZeroPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *SingleEncoder) ResetZeroPosition(ctx context.Context, offset int64, extra map[string]interface{}) error {
	atomic.StoreInt64(&e.position, offset)
	return nil
}
