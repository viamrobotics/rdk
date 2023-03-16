package encoder

/*
	This driver implements a single-wire odometer, such as LM393, as an encoder.
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

import (
	"context"
	"math"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

var singlemodelname = resource.NewDefaultModel("single")

func init() {
	registry.RegisterComponent(
		Subtype,
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
		Subtype,
		singlemodelname,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf SingleWireConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&SingleWireConfig{})
}

// DirectionAware lets you ask what direction something is moving. Only used for SingleEncoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// SingleEncoder keeps track of a motor position using a rotary encoder.
type SingleEncoder struct {
	generic.Unimplemented
	name     string
	I        board.DigitalInterrupt
	position int64
	m        DirectionAware

	logger                  golog.Logger
	CancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// SingleWirePin describes the configuration of Pins for a Single encoder.
type SingleWirePin struct {
	I string `json:"i"`
}

// SingleWireConfig describes the configuration of a single encoder.
type SingleWireConfig struct {
	Pins      SingleWirePin `json:"pins"`
	BoardName string        `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (cfg *SingleWireConfig) Validate(path string) ([]string, error) {
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
func (e *SingleEncoder) AttachDirectionalAwareness(da DirectionAware) {
	e.m = da
}

// NewSingleEncoder creates a new SingleEncoder.
func NewSingleEncoder(
	ctx context.Context,
	deps registry.Dependencies,
	rawConfig config.Component,
	logger golog.Logger,
) (*SingleEncoder, error) {
	cfg, ok := rawConfig.ConvertedAttributes.(*SingleWireConfig)
	if !ok {
		return nil, rutils.NewUnexpectedTypeError(cfg, rawConfig.ConvertedAttributes)
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	e := &SingleEncoder{name: rawConfig.Name, logger: logger, CancelCtx: cancelCtx, cancelFunc: cancelFunc, position: 0}

	board, err := board.FromDependencies(deps, cfg.BoardName)
	if err != nil {
		return nil, err
	}

	e.I, ok = board.DigitalInterruptByName(cfg.Pins.I)
	if !ok {
		return nil, errors.Errorf("cannot find pin (%s) for SingleEncoder", cfg.Pins.I)
	}

	e.Start(ctx)

	return e, nil
}

// Start starts the SingleEncoder background thread.
func (e *SingleEncoder) Start(ctx context.Context) {
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

// TicksCount returns the current position.
func (e *SingleEncoder) TicksCount(ctx context.Context, extra map[string]interface{}) (float64, error) {
	res := atomic.LoadInt64(&e.position)
	return float64(res), nil
}

// Reset sets the current position of the motor (adjusted by a given offset).
func (e *SingleEncoder) Reset(ctx context.Context, offset float64, extra map[string]interface{}) error {
	offsetInt := int64(math.Round(offset))
	atomic.StoreInt64(&e.position, offsetInt)
	return nil
}

// Close shuts down the SingleEncoder.
func (e *SingleEncoder) Close() error {
	e.logger.Debug("Closing SingleEncoder")
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
