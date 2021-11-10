// Package vgripper implements versions of the Viam gripper.
package vgripper

import (
	"context"
	_ "embed" // used to import model frame
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor/forcematrix"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// modelName is used to register the gripper to a model name.
const modelName = "viam-v2"

//go:embed vgripper_model.json
var vgripperjson []byte

func init() {
	registry.RegisterGripper(modelName, registry.Gripper{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
			return New(ctx, r, config, logger)
		},
	})
}

// Parameters for calibration & operating the gripper.
const (
	targetRPM               = 200
	maxCurrent              = 300
	currentBadReadingCounts = 50
	openPosOffset           = 0.4 // Reduce maximum opening width, keeps out of mechanical binding region
	numMeasurementsCalib    = 10  // Number of measurements at each end position taken when calibrating the gripper
	positionTolerance       = 1   // Tolerance for motor position when reaching the open or closed position
)

// GripperV2 represents a Viam gripper which operates with a ForceMatrix.
type GripperV2 struct {
	motor       motor.Motor
	current     board.AnalogReader
	forceMatrix forcematrix.ForceMatrix

	openPos, closedPos float64

	holdingPressure float32

	pressureLimit             float64
	calibrationNoiseThreshold float64

	closedDirection, openDirection pb.DirectionRelative
	logger                         golog.Logger

	frameJSON             []byte
	numBadCurrentReadings int
}

// New returns a GripperV2 which operates with a ForceMatrix.
func New(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*GripperV2, error) {
	boardName := config.Attributes.String("board")
	board, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("%v gripper requires a board called %v", modelName, boardName)
	}

	motorName := config.Attributes.String("motor")
	motor, exists := r.MotorByName(motorName)
	if !exists {
		return nil, errors.Errorf("failed to find motor named '%v'", motorName)
	}

	supported, err := motor.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, errors.New("gripper motor needs to support position")
	}

	currentAnalogReaderName := config.Attributes.String("current")
	current, exists := board.AnalogReaderByName(currentAnalogReaderName)
	if !exists {
		return nil, errors.Errorf("failed to find analog reader named '%v'", currentAnalogReaderName)
	}

	forceMatrixName := config.Attributes.String("forcematrix")
	forceMatrix, exists := r.SensorByName(forceMatrixName)
	if !exists {
		return nil, errors.Errorf("failed to find a forcematrix sensor named '%v'", forceMatrixName)
	}

	forceMatrixDevice, ok := forceMatrix.(forcematrix.ForceMatrix)
	if !ok {
		return nil, errors.Errorf("(%v) is not a ForceMatrix device", forceMatrixName)
	}

	pressureLimit := config.Attributes.Float64("pressureLimit", 30)
	calibrationNoiseThreshold := config.Attributes.Float64("calibrationNoiseThreshold", 7)

	vg := &GripperV2{
		motor:                     motor,
		current:                   current,
		forceMatrix:               forceMatrixDevice,
		pressureLimit:             pressureLimit,
		calibrationNoiseThreshold: calibrationNoiseThreshold,
		holdingPressure:           .5,
		logger:                    logger,
		frameJSON:                 vgripperjson,
	}

	if err := vg.calibrate(ctx, logger); err != nil {
		return nil, err
	}

	if err := vg.Open(ctx); err != nil {
		return nil, err
	}

	return vg, nil
}

// calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *GripperV2) calibrate(ctx context.Context, logger golog.Logger) error {
	// This will be passed to GoTillStop
	stopFuncHighCurrent := func(ctx context.Context) bool {
		current, err := vg.readCurrent(ctx)
		if err != nil {
			logger.Error(err)
			return true
		}

		err = vg.processCurrentReading(ctx, current, "init")
		if err != nil {
			logger.Error(err)
			return true
		}
		return false
	}

	// Test forward motion for pressure/endpoint
	logger.Debug("init: moving forward")
	err := vg.motor.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, targetRPM/2, stopFuncHighCurrent)
	if err != nil {
		return err
	}
	pressure, err := vg.readRobustAveragePressure(ctx, numMeasurementsCalib)
	if err != nil {
		return err
	}
	position, err := vg.motor.Position(ctx)
	if err != nil {
		return err
	}

	var pressureOpen, pressureClosed float64
	if pressure > vg.pressureLimit {
		vg.closedPos = position
		vg.closedDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		pressureOpen = pressure
	}

	// Test backward motion for pressure/endpoint
	logger.Debug("init: moving backward")
	err = vg.motor.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, targetRPM/2, stopFuncHighCurrent)
	if err != nil {
		return err
	}
	pressure, err = vg.readRobustAveragePressure(ctx, numMeasurementsCalib)
	if err != nil {
		return err
	}
	position, err = vg.motor.Position(ctx)
	if err != nil {
		return err
	}
	if pressure > vg.pressureLimit {
		vg.closedPos = position
		vg.closedDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		pressureOpen = pressure
	}

	// Sanity check; if the pressure difference between open & closed position is too small,
	// something went wrong
	if math.Abs(pressureOpen-pressureClosed) < vg.calibrationNoiseThreshold ||
		vg.closedDirection == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED ||
		vg.openDirection == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return errors.Errorf("init: open and closed positions can't be distinguished: "+
			"positions (closed, open): %f %f, pressures (closed, open): %f %f, "+
			"open direction: %v, closed direction: %v",
			vg.closedPos, vg.openPos, pressureClosed, pressureOpen, vg.openDirection, vg.closedDirection)
	}

	if vg.openDirection == vg.closedDirection {
		return errors.New("openDirection and vg.closedDirection have to be opposed")
	}

	if math.Signbit(vg.openPos - vg.closedPos) {
		vg.openPos += openPosOffset
	} else {
		vg.openPos -= openPosOffset
	}

	return nil
}

// ModelFrame returns the json bytes that describe the dynamic frame of the model
func (vg *GripperV2) ModelFrame() []byte {
	return vg.frameJSON
}

// Open opens the jaws.
func (vg *GripperV2) Open(ctx context.Context) error {
	err := vg.Stop(ctx)
	if err != nil {
		return err
	}
	err = vg.motor.GoTo(ctx, targetRPM, vg.openPos)
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
		isOn, err := vg.motor.IsOn(ctx)
		if err != nil {
			return err
		}
		if !isOn {
			measuredPos, err := vg.motor.Position(ctx)
			if err != nil {
				return err
			}
			if math.Abs(measuredPos-vg.openPos) > positionTolerance {
				return errors.Errorf("didn't reach open position, wanted: %f +/- %v, am at: %f", vg.openPos, positionTolerance, measuredPos)

			}
			return nil
		}
		current, err := vg.readCurrent(ctx)
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}
		err = vg.processCurrentReading(ctx, current, "opening")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		total += msPer
		if total > 5000 {
			measuredPos, err := vg.motor.Position(ctx)
			return vg.stopAfterError(ctx, multierr.Combine(errors.Errorf("open timed out, wanted: %f at: %f", vg.openPos, measuredPos), err))
		}
	}
}

// Grab closes the jaws until pressure is sensed and returns true,
// or until closed position is reached, and returns false.
func (vg *GripperV2) Grab(ctx context.Context) (bool, error) {
	err := vg.Stop(ctx)
	if err != nil {
		return false, err
	}
	err = vg.motor.GoTo(ctx, targetRPM, vg.closedPos)
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
		isOn, err := vg.motor.IsOn(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		pressure, _, current, err := vg.analogs(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		if !isOn {
			measuredPos, err := vg.motor.Position(ctx)
			if err != nil {
				return false, err
			}
			if !pressure && math.Abs(measuredPos-vg.closedPos) > positionTolerance {
				return false, errors.Errorf("didn't reach closed position and am not holding an object,"+
					"closed position: %f +/- %v tolerance, actual position: %f", vg.closedPos, positionTolerance, measuredPos)

			}
			return false, nil
		}

		err = vg.processCurrentReading(ctx, current, "grabbing")
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		if pressure {
			now, err := vg.motor.Position(ctx)
			if err != nil {
				return false, err
			}
			vg.logger.Debugf("i think i grabbed something, have pressure, pos: %f closedPos: %v", now, vg.closedPos)
			err = vg.motor.Go(ctx, vg.closedDirection, vg.holdingPressure)
			return err == nil, err
		}

		total += msPer
		if total > 5000 {
			pressureRaw, err := vg.readAveragePressure(ctx)
			if err != nil {
				return false, vg.stopAfterError(ctx, err)
			}
			now, err := vg.motor.Position(ctx)
			if err != nil {
				return false, vg.stopAfterError(ctx, err)
			}
			return false, vg.stopAfterError(ctx, errors.Errorf("close timed out, wanted: %f at: %f pressure: %f",
				vg.closedPos, now, pressureRaw))
		}
	}
}

// processCurrentReading checks if the current is within a healthy range or not.
func (vg *GripperV2) processCurrentReading(ctx context.Context, current int, where string) error {
	if current < maxCurrent {
		vg.numBadCurrentReadings = 0
		return nil
	}
	vg.numBadCurrentReadings++
	if vg.numBadCurrentReadings < currentBadReadingCounts {
		return nil
	}
	return errors.Errorf("current too high for too long, currently %d during %s", current, where)
}

// Close stops the motors.
func (vg *GripperV2) Close() error {
	return vg.Stop(context.Background())
}

// stopAfterError stops the motor and returns the combined errors.
func (vg *GripperV2) stopAfterError(ctx context.Context, other error) error {
	return multierr.Combine(other, vg.motor.Off(ctx))
}

// Stop stops the motors.
func (vg *GripperV2) Stop(ctx context.Context) error {
	return vg.motor.Off(ctx)
}

// readCurrent reads the current.
func (vg *GripperV2) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx)
}

// readRobustAveragePressure reads the pressure multiple times and returns the average over
// all matrix cells and number of measurements.
func (vg *GripperV2) readRobustAveragePressure(ctx context.Context, numMeasurements int) (float64, error) {
	var averagePressure float64
	for i := 0; i < numMeasurements; i++ {
		avgPressure, err := vg.readAveragePressure(ctx)
		if err != nil {
			return 0, err
		}
		averagePressure += avgPressure
	}
	averagePressure /= float64(numMeasurements)
	return averagePressure, nil
}

// readAveragePressure reads the ForceMatrix sensors and returns the average over
// all matrix cells.
func (vg *GripperV2) readAveragePressure(ctx context.Context) (float64, error) {
	matrix, err := vg.forceMatrix.Matrix(ctx)
	if err != nil {
		return 0, err
	}

	sum := 0
	for r := range matrix {
		for _, v := range matrix[r] {
			sum += v
		}
	}
	averagePressure := float64(sum) / float64((len(matrix) * len(matrix[0])))
	return averagePressure, nil
}

// hasPressure checks if the average pressure measurement is above the
// pressureLimit threshold or not.
func (vg *GripperV2) hasPressure(ctx context.Context) (bool, float64, error) {
	p, err := vg.readAveragePressure(ctx)
	if err != nil {
		return false, 0, err
	}
	return p > vg.pressureLimit, p, err
}

// analogs returns measurements such as: boolean that indicates if the average
// pressure is above the pressure limit, the average pressure from the ForceMatrix,
// and the current in the motor.
func (vg *GripperV2) analogs(ctx context.Context) (bool, float64, int, error) {
	hasPressure, pressure, err := vg.hasPressure(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	current, err := vg.readCurrent(ctx)
	if err != nil {
		return false, 0, 0, err
	}

	return hasPressure, pressure, current, nil
}
