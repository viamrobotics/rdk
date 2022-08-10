// Package fake implements a fake encoder.
package fake

import (
	"context"
	"sync"
	"time"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/utils"
)

// Encoder keeps track of a fake motor position.
type Encoder struct {
	mu         			sync.Mutex
	position   			int64
	speed      			float64 // ticks per minute
	updateRate 			int64   // update position in start every updateRate ms
	Tpr					int64	// ticks per rotation
	activeBackgroundWorkers *sync.WaitGroup

	generic.Unimplemented
}

// GetTicksCount returns the current position in terms of ticks.
func (e *Encoder) GetTicksCount(ctx context.Context, extra map[string]interface{}) (int64, error) {
	return e.position, nil
}

// Start starts a background thread to run the encoder.
func (e *Encoder) Start(cancelCtx context.Context, onStart func()) {
	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		if e.updateRate == 0 {
			e.updateRate = 100
		}
		for {
			select {
			case <-cancelCtx.Done():
				return
			default:
			}

			if !utils.SelectContextOrWait(cancelCtx, time.Duration(e.updateRate)*time.Millisecond) {
				return
			}

			e.mu.Lock()
			e.position += int64(e.speed / float64(60*1000/e.updateRate))
			e.mu.Unlock()
		}
	}, e.activeBackgroundWorkers.Done)
}

// ResetToZero resets the zero position.
func (e *Encoder) ResetToZero(ctx context.Context, offset int64, extra map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = offset
	return nil
}

// SetSpeed sets the speed of the fake motor the encoder is measuring.
func (e *Encoder) SetSpeed(ctx context.Context, speed float64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.speed = speed
	return nil
}

// SetPosition sets the position of the encoder.
func (e *Encoder) SetPosition(ctx context.Context, position int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = position
	return nil
}

func (e *Encoder) TicksPerRotation(ctx context.Context) (int64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Tpr, nil
}