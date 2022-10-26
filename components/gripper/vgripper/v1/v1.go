// Package vgripper implements versions of the Viam gripper.
// This is an Experimental package
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

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	rdkutils "go.viam.com/rdk/utils"
)

//go:embed vgripper_model.json
var vgripperv1json []byte

// modelName is used to register the gripper to a model name.
const modelName = "viam-v1"

// AttrConfig is the config for a viam gripper.
type AttrConfig struct {
	Board         string `json:"board"`
	PressureLimit int    `json:"pressure_limit"`
}

// Validate ensures all parts of the config are valid.
func (config *AttrConfig) Validate(path string) ([]string, error) {
	var deps []string
	if config.Board == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if config.PressureLimit == 0 {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "pressure_limit")
	}
	deps = append(deps, config.Board)
	return deps, nil
}

func init() {
	registry.RegisterComponent(gripper.Subtype, modelName, registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			attr, ok := config.ConvertedAttributes.(*AttrConfig)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(attr, config.ConvertedAttributes)
			}
			b, err := board.FromDependencies(deps, attr.Board)
			if err != nil {
				return nil, err
			}
			return newGripperV1(ctx, deps, b, config, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(gripper.SubtypeName, modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf AttrConfig
			return config.TransformAttributeMapToStruct(&conf, attributes)
		}, &AttrConfig{})
}

// TODO.
const (
	TargetRPM               = 200
	MaxCurrent              = 700
	CurrentBadReadingCounts = 50
	MinRotationGap          = 4.0
	MaxRotationGap          = 5.0
	OpenPosOffset           = 0.4 // Reduce maximum opening width, keeps out of mechanical binding region
	ClosePosOffset          = 0.1 // Prevent false "grabs"
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
	opMgr                         operation.SingleOperationManager
	logger                        golog.Logger

	model                 referenceframe.Model
	numBadCurrentReadings int
}

// newGripperV1 Returns a gripperV1.
func newGripperV1(
	ctx context.Context,
	deps registry.Dependencies,
	theBoard board.Board,
	cfg config.Component,
	logger golog.Logger,
) (gripper.LocalGripper, error) {
	attr, ok := cfg.ConvertedAttributes.(*AttrConfig)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(attr, cfg.ConvertedAttributes)
	}
	pressureLimit := attr.PressureLimit
	if pressureLimit == 0 {
		pressureLimit = 800
	}
	_motor, err := motor.FromDependencies(deps, "g")
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
	supportedProperties, err := vg.motor.Properties(ctx, nil)
	if err != nil {
		return nil, err
	}
	supported := supportedProperties[motor.PositionReporting]
	if !supported {
		return nil, motor.NewFeatureUnsupportedError(motor.PositionReporting, "g")
	}

	if vg.current == nil || vg.pressure == nil {
		return nil, errors.New("gripper needs a current and a pressure reader")
	}

	err = vg.Home(ctx)
	if err != nil {
		return nil, err
	}
	return vg, nil
}

func (vg *gripperV1) Home(ctx context.Context) error {
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
			vg.logger.Error(err)
			return true
		}
		err = vg.processCurrentReading(current, "init")
		if err != nil {
			vg.logger.Error(err)
			return true
		}
		pressure, err := vg.readPressure(ctx)
		if err != nil {
			vg.logger.Error(err)
			return true
		}
		if pressure < vg.pressureLimit {
			if localTest.nonPressureSeen {
				localTest.pressureCount++
			}
		} else {
			localTest.nonPressureCount++
			// Capture the last position BEFORE pressure is detected
			localTest.pressurePos, err = vg.motor.Position(ctx, nil)
			if err != nil {
				vg.logger.Error(err)
				return true
			}
		}
		if localTest.nonPressureCount == 15 {
			localTest.nonPressureSeen = true
			vg.logger.Debug("init: non-pressure range found")
		}
		if localTest.pressureCount >= 5 {
			vg.logger.Debug("init: pressure sensing (closed) direction found")
			localTest.pressureSeen = true
			return true
		}
		return false
	}

	// Test forward motion for pressure/endpoint
	vg.logger.Debug("init: moving forward")
	err := vg.motor.GoTillStop(ctx, TargetRPM/2, stopFunc)
	if err != nil {
		return err
	}
	if localTest.pressureSeen {
		hasPressureA = true
		posA = localTest.pressurePos
	} else {
		posA, err = vg.motor.Position(ctx, nil)
		if err != nil {
			return err
		}
	}
	// Test backward motion for pressure/endpoint
	localTest = &movementTest{}
	vg.logger.Debug("init: moving backward")
	err = vg.motor.GoTillStop(ctx, -1*TargetRPM/2, stopFunc)
	if err != nil {
		return err
	}
	if localTest.pressureSeen {
		hasPressureB = true
		posB = localTest.pressurePos
	} else {
		posB, err = vg.motor.Position(ctx, nil)
		if err != nil {
			return err
		}
	}

	// One final movement, in the case that we start closed AND the first movement was also toward closed (no non-pressure range seen)
	if !hasPressureA && !hasPressureB {
		localTest = &movementTest{}
		vg.logger.Debug("init: moving forward (2nd try)")
		err = vg.motor.GoTillStop(ctx, TargetRPM/2, stopFunc)
		if err != nil {
			return err
		}
		if localTest.pressureSeen {
			hasPressureA = true
			posA = localTest.pressurePos
		} else {
			posA, err = vg.motor.Position(ctx, nil)
			if err != nil {
				return err
			}
		}
	}

	if hasPressureA == hasPressureB {
		return errors.Errorf(
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
		vg.closePos -= ClosePosOffset
	} else {
		vg.openPos -= OpenPosOffset
		vg.closePos += ClosePosOffset
	}

	vg.logger.Debugf("init: orig openPos: %f, closePos: %f", vg.openPos, vg.closePos)
	// Zero to closed position
	curPos, err := vg.motor.Position(ctx, nil)
	if err != nil {
		return err
	}
	err = vg.motor.ResetZeroPosition(ctx, curPos-vg.closePos, nil)
	if err != nil {
		return err
	}
	vg.openPos -= vg.closePos
	vg.closePos = 0

	vg.logger.Debugf("init: final openPos: %f, closePos: %f", vg.openPos, vg.closePos)
	rotationGap := math.Abs(vg.openPos - vg.closePos)
	if rotationGap < MinRotationGap || rotationGap > MaxRotationGap {
		return errors.Errorf(
			"init: rotationGap not in expected range got: %v range %v -> %v",
			rotationGap,
			MinRotationGap,
			MaxRotationGap,
		)
	}

	return vg.Open(ctx)
}

// ModelFrame returns the dynamic frame of the model.
func (vg *gripperV1) ModelFrame() referenceframe.Model {
	return vg.model
}

// Open opens the jaws.
func (vg *gripperV1) Open(ctx context.Context) error {
	ctx, done := vg.opMgr.New(ctx)
	defer done()

	err := vg.Stop(ctx)
	if err != nil {
		return err
	}

	pos, err := vg.motor.Position(ctx, nil)
	if err != nil {
		return err
	}

	if math.Abs(pos-vg.openPos) < 0.1 {
		return nil
	}

	utils.PanicCapturingGo(func() {
		err := vg.motor.GoTo(ctx, TargetRPM, vg.openPos, nil)
		if err != nil {
			vg.logger.Error(err)
		}
	})

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to open
		isOn, _, err := vg.motor.IsPowered(ctx, nil)
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
			now, err := vg.motor.Position(ctx, nil)
			return vg.stopAfterError(ctx, multierr.Combine(errors.Errorf("open timed out, wanted: %f at: %f", vg.openPos, now), err))
		}
	}
}

// Grab closes the jaws until pressure is sensed and returns true, or until closed position is reached, and returns false.
func (vg *gripperV1) Grab(ctx context.Context) (bool, error) {
	ctx, done := vg.opMgr.New(ctx)
	defer done()

	err := vg.Stop(ctx)
	if err != nil {
		return false, err
	}

	pos, err := vg.motor.Position(ctx, nil)
	if err != nil {
		return false, err
	}

	if math.Abs(pos-vg.closePos) < 0.1 {
		return false, nil
	}

	utils.PanicCapturingGo(func() {
		err := vg.motor.GoTo(ctx, TargetRPM, vg.closePos, nil)
		if err != nil {
			vg.logger.Error(err)
		}
	})

	msPer := 10
	total := 0
	for {
		if !utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond) {
			return false, vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to closed
		isOn, _, err := vg.motor.IsPowered(ctx, nil)
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
			now, err := vg.motor.Position(ctx, nil)
			if err != nil {
				return false, err
			}
			vg.logger.Debugf("i think i grabbed something, have pressure, pos: %f closePos: %v", now, vg.closePos)
			err = vg.motor.SetPower(ctx, float64(vg.closeDirection)*vg.holdingPressure, nil)
			return true, err
		}

		total += msPer
		if total > 5000 {
			pressureRaw, err := vg.readPressure(ctx)
			if err != nil {
				return false, vg.stopAfterError(ctx, err)
			}
			now, err := vg.motor.Position(ctx, nil)
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
	return multierr.Combine(other, vg.motor.Stop(ctx, nil))
}

// Stop stops the motors.
func (vg *gripperV1) Stop(ctx context.Context) error {
	ctx, done := vg.opMgr.New(ctx)
	defer done()
	return vg.motor.Stop(ctx, nil)
}

// IsMoving returns whether the gripper is moving.
func (vg *gripperV1) IsMoving(ctx context.Context) (bool, error) {
	// RSDK-434: Refine implementation
	return vg.opMgr.OpRunning(), nil
}

func (vg *gripperV1) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx, nil)
}

func (vg *gripperV1) readPressure(ctx context.Context) (int, error) {
	return vg.pressure.Read(ctx, nil)
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

// Do() related constants.
const (
	Command           = "command"
	GetPressure       = "get_pressure"
	GetCurrent        = "get_current"
	Home              = "home"
	ReturnCurrent     = "current"
	ReturnPressure    = "pressure"
	ReturnHasPressure = "has_pressure"
)

func (vg *gripperV1) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	name, ok := cmd[Command]
	if !ok {
		return nil, errors.Errorf("missing %s value", Command)
	}
	switch name {
	case GetPressure:
		hasPressure, pressure, err := vg.hasPressure(ctx)
		return map[string]interface{}{ReturnHasPressure: hasPressure, ReturnPressure: pressure}, err
	case GetCurrent:
		current, err := vg.readCurrent(ctx)
		return map[string]interface{}{ReturnCurrent: current}, err
	case Home:
		return nil, vg.Home(ctx)
	default:
		return nil, errors.Errorf("no such command: %s", name)
	}
}
