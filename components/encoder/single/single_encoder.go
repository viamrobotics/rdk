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
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

const (
	isSingle          = "single"
	directionAttached = "direction"
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

	workers *utils.StoppableWorkers
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
func (conf *Config) Validate(path string) ([]string, []string, error) {
	var deps []string

	if conf.Pins.I == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "i")
	}

	if len(conf.BoardName) == 0 {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, conf.BoardName)

	return deps, nil, nil
}

// AttachDirectionalAwareness to pre-created encoder.
func (e *Encoder) AttachDirectionalAwareness(da DirectionAware) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.m = da
}

// NewSingleEncoder creates a new Encoder.
func NewSingleEncoder(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (encoder.Encoder, error) {
	e := &Encoder{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
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
	e.mu.Lock()
	defer e.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	existingBoardName := e.boardName
	existingDIPinName := e.diPinName

	needRestart := existingBoardName != newConf.BoardName ||
		existingDIPinName != newConf.Pins.I

	board, err := board.FromDependencies(deps, newConf.BoardName)
	if err != nil {
		return err
	}

	di, err := board.DigitalInterruptByName(newConf.Pins.I)
	if err != nil {
		return multierr.Combine(errors.Errorf("cannot find pin (%s) for Encoder", newConf.Pins.I), err)
	}

	if !needRestart {
		return nil
	}

	e.I = di
	e.boardName = newConf.BoardName
	e.diPinName = newConf.Pins.I
	// state is not really valid anymore
	atomic.StoreInt64(&e.position, 0)

	if e.workers != nil {
		e.workers.Stop() // Shut down the old interrupt stream
	}
	e.start(board) // Start up the new interrupt stream
	return nil
}

// start starts the Encoder background thread. It should only be called when the encoder's
// background workers have been stopped (or never started).
func (e *Encoder) start(b board.Board) {
	e.workers = utils.NewBackgroundStoppableWorkers()

	encoderChannel := make(chan board.Tick)
	err := b.StreamTicks(e.workers.Context(), []board.DigitalInterrupt{e.I}, encoderChannel, nil)
	if err != nil {
		utils.Logger.Errorw("error getting interrupt ticks", "error", err)
		return
	}

	e.workers.Add(func(cancelCtx context.Context) {
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

			if e.m != nil {
				// There is a minor race condition here. Delays in interrupt processing may result in a
				// DirectionMoving() value that is *currently* different from one that was used to drive
				// the motor. This may result in ticks being lost or applied in the wrong direction.
				dir := e.m.DirectionMoving()
				if dir == 1 || dir == -1 {
					atomic.AddInt64(&e.position, dir)
				}
			} else {
				// if no motor is attached to the encoder, increase in positive direction.
				e.logger.Debug("no motor is attached to the encoder, increasing ticks count in the positive direction only")
				atomic.AddInt64(&e.position, 1)
			}
		}
	})
}

// Position returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (e *Encoder) Position(
	ctx context.Context,
	positionType encoder.PositionType,
	extra map[string]interface{},
) (float64, encoder.PositionType, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

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
	e.mu.Lock()
	defer e.mu.Unlock()

	// In unit tests, we construct encoders without calling NewSingleEncoder(), which means they
	// might not have called e.start(), so might not have initialized e.workers. Don't crash if
	// that happens.
	if e.workers != nil {
		e.workers.Stop() // This also shuts down the interrupt stream.
	}
	return nil
}

// DoCommand uses a map string to run custom functionality of a single encoder.
func (e *Encoder) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	resp := make(map[string]interface{})

	if m, ok := cmd[isSingle].(motor.Motor); ok {
		e.AttachDirectionalAwareness(m.(DirectionAware))
		resp[directionAttached] = true
	}

	return resp, nil
}
