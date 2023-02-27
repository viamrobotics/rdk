// Package fake implements a fake encoder.
package fake

import (
	"context"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var fakeModel = resource.NewDefaultModel("fake")

func init() {
	_encoder := registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			e := &Encoder{}
			e.updateRate = cfg.ConvertedAttributes.(*AttrConfig).UpdateRate

			e.Start(ctx)
			return e, nil
		},
	}
	registry.RegisterComponent(encoder.Subtype, fakeModel, _encoder)

	config.RegisterComponentAttributeMapConverter(
		encoder.Subtype,
		fakeModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		}, &AttrConfig{})
}

// AttrConfig describes the configuration of a fake encoder.
type AttrConfig struct {
	UpdateRate int64 `json:"update_rate_msec"`
}

// Validate ensures all parts of a config is valid.
func (cfg *AttrConfig) Validate(path string) error {
	return nil
}

// Encoder keeps track of a fake motor position.
type Encoder struct {
	mu                      sync.Mutex
	position                int64
	speed                   float64 // ticks per minute
	updateRate              int64   // update position in start every updateRate ms
	activeBackgroundWorkers sync.WaitGroup

	generic.Unimplemented
}

// TicksCount returns the current position in terms of ticks.
func (e *Encoder) TicksCount(ctx context.Context, extra map[string]interface{}) (float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return float64(e.position), nil
}

// Start starts a background thread to run the encoder.
func (e *Encoder) Start(cancelCtx context.Context) {
	if e.updateRate == 0 {
		e.mu.Lock()
		e.updateRate = 100
		e.mu.Unlock()
	}

	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
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

// Reset sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *Encoder) Reset(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if err := encoder.ValidateIntegerOffset(offset); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = int64(offset)
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
