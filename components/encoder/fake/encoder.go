// Package fake implements a fake encoder.
package fake

import (
	"context"
	"errors"
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
			return newFakeEncoder(ctx, cfg)
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

// newFakeEncoder creates a new Encoder.
//
//nolint:unparam
func newFakeEncoder(
	ctx context.Context,
	cfg config.Component,
) (encoder.Encoder, error) {
	e := &Encoder{
		position:     0,
		positionType: encoder.PositionTypeTICKS,
	}
	e.updateRate = cfg.ConvertedAttributes.(*AttrConfig).UpdateRate

	e.Start(ctx)
	return e, nil
}

// AttrConfig describes the configuration of a fake encoder.
type AttrConfig struct {
	UpdateRate int64 `json:"update_rate_msec,omitempty"`
}

// Validate ensures all parts of a config is valid.
func (cfg *AttrConfig) Validate(path string) error {
	return nil
}

// Encoder keeps track of a fake motor position.
type Encoder struct {
	mu                      sync.Mutex
	position                int64
	positionType            encoder.PositionType
	speed                   float64 // ticks per minute
	updateRate              int64   // update position in start every updateRate ms
	activeBackgroundWorkers sync.WaitGroup

	generic.Unimplemented
}

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) GetPosition(
	ctx context.Context,
	positionType *encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType != nil && *positionType == encoder.PositionTypeDEGREES {
		err := errors.New("Encoder does not support PositionType Angle Degrees, use a different PositionType")
		return 0, *positionType, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return float64(e.position), e.positionType, nil
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

// ResetPosition sets the current position of the motor (adjusted by a given offset)
// to be its new zero position.
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.position = int64(0)
	return nil
}

// GetProperties returns a list of all the position types that are supported by a given encoder.
func (e *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	return map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: false,
	}, nil
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
