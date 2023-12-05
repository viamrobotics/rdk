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

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var singleModel = resource.DefaultModelFamily.WithModel("single")

func init() {
	resource.RegisterComponent(
		encoder.API,
		singleModel,
		resource.Registration[encoder.Encoder, *Config]{
			Constructor: NewSingleEncoder,
		})
}

// DirectionAware lets you ask what direction something is moving. Only used for Encoder for now, unclear future.
// DirectionMoving returns -1 if the motor is currently turning backwards, 1 if forwards and 0 if off.
type DirectionAware interface {
	DirectionMoving() int64
}

// Encoder keeps track of a motor position using a rotary encoder.s.
type Encoder struct {
	resource.Named

	position int64

	mu        sync.Mutex
	I         board.DigitalInterrupt
	m         DirectionAware
	boardName string
	diPinName string

	positionType encoder.PositionType
	logger       logging.Logger

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// Pin describes the configuration of Pins for a Single encoder.
type Pin struct {
	I string `json:"i"`
}

// Config describes the configuration of a single encoder.
type Config struct {
	Pins      Pin    `json:"pins"`
	BoardName string `json:"board"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.Pins.I == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "i")
	}

	if len(conf.BoardName) == 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, conf.BoardName)

	return deps, nil
}

// AttachDirectionalAwareness to pre-created encoder.
func (e *Encoder) AttachDirectionalAwareness(da DirectionAware) {
	e.mu.Lock()
	e.m = da
	e.mu.Unlock()
}

// NewSingleEncoder creates a new Encoder.
func NewSingleEncoder(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (encoder.Encoder, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	e := &Encoder{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		position:     0,
		positionType: encoder.PositionTypeTicks,
	}
	if err := e.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return e, nil
}

// Reconfigure atomically reconfigures this encoder in place based on the new config.
func (e *Encoder) Reconfigure(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	e.mu.Lock()
	existingBoardName := e.boardName
	existingDIPinName := e.diPinName
	e.mu.Unlock()

	needRestart := existingBoardName != newConf.BoardName ||
		existingDIPinName != newConf.Pins.I

	board, err := board.FromDependencies(deps, newConf.BoardName)
	if err != nil {
		return err
	}

	di, ok := board.DigitalInterruptByName(newConf.Pins.I)
	if !ok {
		return errors.Errorf("cannot find pin (%s) for Encoder", newConf.Pins.I)
	}

	if !needRestart {
		return nil
	}
	utils.UncheckedError(e.Close(ctx))
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	e.cancelCtx = cancelCtx
	e.cancelFunc = cancelFunc

	e.mu.Lock()
	e.I = di
	e.boardName = newConf.BoardName
	e.diPinName = newConf.Pins.I
	// state is not really valid anymore
	atomic.StoreInt64(&e.position, 0)
	e.mu.Unlock()

	e.Start(ctx)

	return nil
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
			case <-e.cancelCtx.Done():
				return
			default:
			}

			select {
			case <-e.cancelCtx.Done():
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
				e.logger.Debug("received tick for encoder that isn't connected to a motor; ignoring")
			}
		}
	}, e.activeBackgroundWorkers.Done)
}

// Position returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) Position(
	ctx context.Context,
	positionType encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	if positionType == encoder.PositionTypeDegrees {
		return math.NaN(), encoder.PositionTypeUnspecified, encoder.NewPositionTypeUnsupportedError(positionType)
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

// Properties returns a list of all the position types that are supported by a given encoder.
func (e *Encoder) Properties(ctx context.Context, extra map[string]interface{}) (encoder.Properties, error) {
	return encoder.Properties{
		TicksCountSupported:   true,
		AngleDegreesSupported: false,
	}, nil
}

// Close shuts down the Encoder.
func (e *Encoder) Close(ctx context.Context) error {
	e.cancelFunc()
	e.activeBackgroundWorkers.Wait()
	return nil
}
