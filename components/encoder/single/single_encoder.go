/*
Package single implements a single-wire odometer, such as LM393, as an encoder.
This allows the attached motor to determine its relative position.
This class of encoders requires a single digital interrupt pin.

This encoder must be connected to a motor (or another component that supports encoders
and reports the direction it is moving) in order to record readings.
The motor indicates in which direction it is spinning, thus indicating if the encoder
should increment or decrement reading value.

Resetting a position must set the position to an int64. A floating point input will be rounded.

Sample configuration:

	{
		"pins" : {
			"i": 10
		},
		"board": "pi"
	}
*/
package single

import (
	"context"
	"math"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

var singlemodelname = resource.NewDefaultModel("single")

func init() {
	registry.RegisterComponent(
		encoder.Subtype,
		singlemodelname,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewSingleEncoder(ctx, deps, config, logger)
		}})

	config.RegisterComponentAttributeMapConverter(
		encoder.Subtype,
		singlemodelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&AttrConfig{})
}

// DirectionAware lets you ask what direction something is moving. Only used for Encoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// Encoder keeps track of a motor position using a rotary encoder.s.
type Encoder struct {
	generic.Unimplemented
	name     string
	I        board.DigitalInterrupt
	position int64
	m        DirectionAware

	positionType            encoder.PositionType
	logger                  golog.Logger
	CancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Pin describes the configuration of Pins for a Single encoder.
type Pin struct {
	I string `json:"i"`
}

// AttrConfig describes the configuration of a single encoder.
type AttrConfig struct {
	Pins      Pin    `json:"pins"`
	BoardName string `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string

	if cfg.Pins.I == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "i")
	}

	if len(cfg.BoardName) == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, cfg.BoardName)

	return deps, nil
}

// AttachDirectionalAwareness to pre-created encoder.
func (e *Encoder) AttachDirectionalAwareness(da DirectionAware) {
	e.m = da
}

// NewSingleEncoder creates a new Encoder.
func NewSingleEncoder(
	ctx context.Context,
	deps registry.Dependencies,
	rawConfig config.Component,
	logger golog.Logger,
) (encoder.Encoder, error) {
	cfg, ok := rawConfig.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rutils.NewUnexpectedTypeError(cfg, rawConfig.ConvertedAttributes)
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &Encoder{
		name:         rawConfig.Name,
		logger:       logger,
		CancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		position:     0,
		positionType: encoder.PositionTypeTICKS,
	}

	board, err := board.FromDependencies(deps, cfg.BoardName)
	if err != nil {
		return nil, err
	}

	e.I, ok = board.DigitalInterruptByName(cfg.Pins.I)
	if !ok {
		return nil, errors.Errorf("cannot find pin (%s) for Encoder", cfg.Pins.I)
	}

	e.Start(ctx)

	return e, nil
}

// Start starts the Encoder background thread.
func (e *Encoder) Start(ctx context.Context) {
	encoderChannel := make(chan board.Tick)
	e.I.AddCallback(encoderChannel)
	e.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		defer e.I.RemoveCallback(encoderChannel)
		for {
			select {
			case <-e.CancelCtx.Done():
				return
			default:
			}

			select {
			case <-e.CancelCtx.Done():
				return
			case <-encoderChannel:
			}

			if e.m != nil {
				// There is a minor race condition here. Delays in interrupt processing may result in a
				// DirectionMoving() value that is *currently* different from one that was used to drive
				// the motor. This may result in ticks being lost or applied in the wrong direction.
				dir := e.m.DirectionMoving()
				if dir == 1 || dir == -1 {
					atomic.AddInt64(&e.position, dir)
				}
			} else {
				e.logger.Warn("%s: received tick for encoder that isn't connected to a motor, ignoring.", e.name)
			}
		}
	}, e.activeBackgroundWorkers.Done)
}

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) GetPosition(
	ctx context.Context,
	positionType *encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType != nil && *positionType == encoder.PositionTypeDEGREES {
		err := encoder.NewEncoderTypeUnsupportedError(*positionType)
		return 0, *positionType, err
	}
	res := atomic.LoadInt64(&e.position)
	return float64(res), e.positionType, nil
}

// ResetPosition sets the current position of the motor (adjusted by a given offset).
func (e *Encoder) ResetPosition(ctx context.Context, extra map[string]interface{}) error {
	offsetInt := int64(math.Round(0))
	atomic.StoreInt64(&e.position, offsetInt)
	return nil
}

// GetProperties returns a list of all the position types that are supported by a given encoder.
func (e *Encoder) GetProperties(ctx context.Context, extra map[string]interface{}) (map[encoder.Feature]bool, error) {
	return map[encoder.Feature]bool{
		encoder.TicksCountSupported:   true,
		encoder.AngleDegreesSupported: false,
	}, nil
}

// Close shuts down the Encoder.
func (e *Encoder) Close() error {
	e.logger.Debug("Closing Encoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
