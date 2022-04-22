// Package vgripper implements versions of the Viam gripper.
package vgripper

import (
	"context"

	// used to import model referenceframe.
	_ "embed"
	"math"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

//go:embed vgripper_model.json
var vgripperv1json []byte

// modelName is used to register the gripper to a model name.
const modelName = "viam-v1"

func init() {
	registry.RegisterComponent(gripper.Subtype, modelName, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			const boardName = "local"
			b, err := board.FromRobot(r, boardName)
			if err != nil {
				return nil, err
			}
			return newGripperV1(ctx, r, b, config, logger)
		},
	})
}

// TODO.
const (
	TargetRPM               = 200
	MaxCurrent              = 500
	CurrentBadReadingCounts = 50
	MinRotationGap          = 4.0
	MaxRotationGap          = 5.0
	OpenPosOffset           = 0.4 // Reduce maximum opening width, keeps out of mechanical binding region
)

// gripperV1 represents a Viam gripper with a single force sensor cell.
type gripperV1 struct {
	motor    motor.LocalMotor
	current  board.AnalogReader
	pressure board.AnalogReader

	openPos, closePos float64

	holdingPressure float64

	pressureLimit int

	closeDirection, openDirection int64
	logger                        golog.Logger

	model                 referenceframe.Model
	numBadCurrentReadings int
}

// newGripperV1 Returns a gripperV1.
func newGripperV1(ctx context.Context, r robot.Robot, theBoard board.Board, cfg config.Component, logger golog.Logger) (*gripperV1, error) {
	pressureLimit := cfg.Attributes.Int("pressure_limit", 800)
	_motor, err := motor.FromRobot(r, "g")
	if err != nil {
		return nil, err
	}
	stoppableMotor, ok := _motor.(motor.LocalMotor)
	if !ok {
		return nil, motor.NewGoTillStopUnsupportedError("g")
	}
	current, ok := theBoard.AnalogReaderByName("current")
	if !ok {
		return nil, errors.New("failed to find analog reader 'current'")
	}
	pressure, ok := theBoard.AnalogReaderByName("pressure")
	if !ok {
		return nil, errors.New("failed to find analog reader 'pressure'")
	}

	model, err := referenceframe.UnmarshalModelJSON(vgripperv1json, "")
	if err != nil {
		return nil, err
	}

	vg := &gripperV1{
		motor:           stoppableMotor,
		current:         current,
		pressure:        pressure,
		holdingPressure: .5,
		pressureLimit:   pressureLimit,
		logger:          logger,
		model:           model,
	}

	if vg.motor == nil {
		return nil, errors.New("gripper needs a motor named 'g'")
	}
	supportedFeatures, err := vg.motor.GetFeatures(ctx)
	if err != nil {
		return nil, err
	}
	supported := supportedFeatures[motor.PositionReporting]
	if !supported {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, "g")
	}

	if vg.current == nil || vg.pressure == nil {
		return nil, errors.New("gripper needs a current and a pressure reader")
	}

	// Variables for the overall init process
	var posA, posB float64
	var hasPressureA, hasPressureB bool

	// Variables to be reset between each movement/test
	type movementTest struct {
		pressureSeen, nonPressureSeen   bool
		pressurePos                     float64
		pressureCount, nonPressureCount int
	}

	localTest := &movementTest{}
	// This will be passed to GoTillStop
	stopFunc := func(ctx context.Context) bool {
		current, err := vg.readCurrent(ctx)
		if err != nil {
			logger.Error(err)
			return true
		}
		err = vg.processCurrentReading(current, "init")
		if err != nil {
			logger.Error(err)
			return true
		}
		pressure, err := vg.readPressure(ctx)
		if err != nil {
			logger.Error(err)
			return true
		}
		if pressure < pressureLimit {
			if localTest.nonPressureSeen {
				localTest.pressureCount++
			}
		} else {
			localTest.nonPressureCount++
			// Capture the last position BEFORE pressure is detected
			localTest.pressurePos, err = vg.motor.GetPosition(ctx)
			if err != nil {
				logger.Error(err)
				return true
			}
		}
		if localTest.nonPressureCount == 15 {
			localTest.nonPressureSeen = true
			logger.Debug("init: non-pressure range found")
		}
		if localTest.pressureCount >= 5 {
			logger.Debug("init: pressure sensing (closed) direction found")
			localTest.pressureSeen = true
			return true
		}
		return false
	}

	// Test forward motion for pressure/endpoint
	logger.Debug("init: moving forward")
	err = vg.motor.GoTillStop(ctx, TargetRPM/2, stopFunc)
	if err != nil {
		return nil, err
	}
	if localTest.pressureSeen {
		hasPressureA = true
		posA = localTest.pressurePos
	} else {
		posA, err = vg.motor.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
	}
	// Test backward motion for pressure/endpoint
	localTest = &movementTest{}
	logger.Debug("init: moving backward")
	err = vg.motor.GoTillStop(ctx, -1*TargetRPM/2, stopFunc)
	if err != nil {
		return nil, err
	}
	if localTest.pressureSeen {
		hasPressureB = true
		posB = localTest.pressurePos
	} else {
		posB, err = vg.motor.GetPosition(ctx)
		if err != nil {
			return nil, err
		}
	}

	// One final movement, in the case that we start closed AND the first movement was also toward closed (no non-pressure range seen)
	if !hasPressureA && !hasPressureB {
		localTest = &movementTest{}
		logger.Debug("init: moving forward (2nd try)")
		err = vg.motor.GoTillStop(ctx, TargetRPM/2, stopFunc)
		if err != nil {
			return nil, err
		}
		if localTest.pressureSeen {
			hasPressureA = true
			posA = localTest.pressurePos
		} else {
			posA, err = vg.motor.GetPosition(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	if hasPressureA == hasPressureB {
		return nil,
			errors.Errorf(
				"init: pressure same open and closed, something is wrong, positions: %f %f, pressures: %t %t",
				posA,
				posB,
				hasPressureA,
				hasPressureB,
			)
	}

	if hasPressureA {
		vg.closeDirection = 1
		vg.openDirection = -1
		vg.openPos = posB
		vg.closePos = posA
	} else {
		vg.closeDirection = -1
		vg.openDirection = 1
		vg.openPos = posA
		vg.closePos = posB
	}

	if math.Signbit(vg.openPos - vg.closePos) {
		vg.openPos += OpenPosOffset
	} else {
		vg.openPos -= OpenPosOffset
	}

	logger.Debugf("init: orig openPos: %f, closePos: %f", vg.openPos, vg.closePos)
	// Zero to closed position
	curPos, err := vg.motor.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	err = vg.motor.ResetZeroPosition(ctx, curPos-vg.closePos)
	if err != nil {
		return nil, err
	}
	vg.openPos -= vg.closePos
	vg.closePos = 0

	logger.Debugf("init: final openPos: %f, closePos: %f", vg.openPos, vg.closePos)
	rotationGap := math.Abs(vg.openPos - vg.closePos)
	if rotationGap < MinRotationGap || rotationGap > MaxRotationGap {
		return nil, errors.Errorf(
			"init: rotationGap not in expected range got: %v range %v -> %v",
			rotationGap,
			MinRotationGap,
			MaxRotationGap,
		)
	}

	return vg, vg.Open(ctx)
}

// ModelFrame returns the dynamic frame of the model.
func (vg *gripperV1) ModelFrame() referenceframe.Model {
	return vg.model
}

// Open opens the jaws.
func (vg *gripperV1) Open(ctx context.Context) error {
	err := vg.Stop(ctx)
	if err != nil {
		return err
	}

	err = vg.motor.GoTo(ctx, TargetRPM, vg.openPos)
	if err != nil {
		return err
	}

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to open
		isOn, err := vg.motor.IsPowered(ctx)
		if err != nil {
			return err
		}
		if !isOn {
			return nil
		}
		current, err := vg.readCurrent(ctx)
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}
		err = vg.processCurrentReading(current, "opening")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		total += msPer
		if total > 5000 {
			now, err := vg.motor.GetPosition(ctx)
			return vg.stopAfterError(ctx, multierr.Combine(errors.Errorf("open timed out, wanted: %f at: %f", vg.openPos, now), err))
		}
	}
}

// Grab closes the jaws until pressure is sensed and returns true, or until closed position is reached, and returns false.
func (vg *gripperV1) Grab(ctx context.Context) (bool, error) {
	err := vg.Stop(ctx)
	if err != nil {
		return false, err
	}
	err = vg.motor.GoTo(ctx, TargetRPM, vg.closePos)
	if err != nil {
		return false, err
	}

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return false, vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to closed
		isOn, err := vg.motor.IsPowered(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}
		if !isOn {
			return false, nil
		}

		pressure, _, current, err := vg.analogs(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}
		err = vg.processCurrentReading(current, "grabbing")
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		if pressure {
			now, err := vg.motor.GetPosition(ctx)
			if err != nil {
				return false, err
			}
			vg.logger.Debugf("i think i grabbed something, have pressure, pos: %f closePos: %v", now, vg.closePos)
			err = vg.motor.SetPower(ctx, float64(vg.closeDirection)*vg.holdingPressure)
			return true, err
		}

		total += msPer
		if total > 5000 {
			pressureRaw, err := vg.readPressure(ctx)
			if err != nil {
				return false, vg.stopAfterError(ctx, err)
			}
			now, err := vg.motor.GetPosition(ctx)
			if err != nil {
				return false, vg.stopAfterError(ctx, err)
			}
			return false, vg.stopAfterError(
				ctx,
				errors.Errorf("close timed out, wanted: %f at: %f pressure: %d", vg.closePos, now, pressureRaw),
			)
		}
	}
}

func (vg *gripperV1) processCurrentReading(current int, where string) error {
	if current < MaxCurrent {
		vg.numBadCurrentReadings = 0
		return nil
	}
	vg.numBadCurrentReadings++
	if vg.numBadCurrentReadings < CurrentBadReadingCounts {
		return nil
	}
	return errors.Errorf("current too high for too long, currently %d during %s", current, where)
}

// Close stops the motors.
func (vg *gripperV1) Close(ctx context.Context) error {
	return vg.Stop(ctx)
}

func (vg *gripperV1) stopAfterError(ctx context.Context, other error) error {
	return multierr.Combine(other, vg.motor.Stop(ctx))
}

// Stop stops the motors.
func (vg *gripperV1) Stop(ctx context.Context) error {
	return vg.motor.Stop(ctx)
}

func (vg *gripperV1) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx)
}

func (vg *gripperV1) readPressure(ctx context.Context) (int, error) {
	return vg.pressure.Read(ctx)
}

func (vg *gripperV1) hasPressure(ctx context.Context) (bool, int, error) {
	p, err := vg.readPressure(ctx)
	return p < vg.pressureLimit, p, err
}

func (vg *gripperV1) analogs(ctx context.Context) (hasPressure bool, pressure, current int, err error) {
	hasPressure, pressure, err = vg.hasPressure(ctx)
	if err != nil {
		return
	}

	current, err = vg.readCurrent(ctx)
	if err != nil {
		return
	}

	return
}

// Do is unimplemented.
func (vg *gripperV1) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("Do() unimplemented")
}
