// Package vgripper implements versions of the Viam gripper.
package vgripper

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor/forcematrix"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
)

// ModelName is used to register the gripper to a model name.
const ModelName = "viam-v2"

func init() {
	registry.RegisterGripper(ModelName, registry.Gripper{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
			return NewGripperV2(ctx, r, config, logger)
		},
		Frame: func(name string) (referenceframe.Frame, error) {
			// A viam gripper is 220mm from mount point to center of gripper paddles
			return referenceframe.FrameFromPoint(name, r3.Vector{0, 0, 220})
		},
	})
}

// Parameters for calibration & operating the gripper.
const (
	TargetRPM               = 200
	MaxCurrent              = 300
	CurrentBadReadingCounts = 50
	MinRotationGap          = 4.0
	MaxRotationGap          = 5.0
	OpenPosOffset           = 0.4 // Reduce maximum opening width, keeps out of mechanical binding region
	numMeasurements         = 10  // Number of measurements at each end position taken when calibrating the gripper
)

// GripperV2 represents a Viam gripper.
type GripperV2 struct {
	motor       motor.Motor
	current     board.AnalogReader
	forceMatrix forcematrix.ForceMatrix

	openPos, closedPos float64

	holdingPressure float32

	pressureLimit float64

	closeDirection, openDirection pb.DirectionRelative
	logger                        golog.Logger

	numBadCurrentReadings int
}

// NewGripperV2 returns a GripperV2.
func NewGripperV2(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*GripperV2, error) {
	const boardName = "local"
	const motorName = "g"
	const currentAnalogReaderName = "current"

	board, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("%v gripper requires a board called %v", ModelName, boardName)
	}

	motor, exists := r.MotorByName(motorName)
	if !exists {
		return nil, errors.Errorf("failed to find motor named '%v'", motorName)
	}

	supported, err := motor.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, errors.Errorf("gripper motor needs to support position")
	}

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
		return nil, errors.Errorf("(%v) is not a forceMatrix device", forceMatrixName)
	}

	pressureLimit := config.Attributes.Float64("pressureLimit", 30)

	vg := &GripperV2{
		motor:           motor,
		current:         current,
		forceMatrix:     forceMatrixDevice,
		pressureLimit:   pressureLimit,
		holdingPressure: .5,
		logger:          logger,
	}

	err = vg.calibrate(ctx, logger)
	if err != nil {
		return nil, err
	}

	return vg, vg.Open(ctx)
}

// calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *GripperV2) calibrate(ctx context.Context, logger golog.Logger) error {
	var pressureOpen, pressureClosed float64

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
	err := vg.motor.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD, TargetRPM/2, stopFuncHighCurrent)
	if err != nil {
		return err
	}
	pressure, err := vg.readRobustAveragePressure(ctx, numMeasurements)
	if err != nil {
		return err
	}
	position, err := vg.motor.Position(ctx)
	if err != nil {
		return err
	}
	if pressure > vg.pressureLimit {
		vg.closedPos = position
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		pressureOpen = pressure
	}

	// Test backward motion for pressure/endpoint
	logger.Debug("init: moving backward")
	err = vg.motor.GoTillStop(ctx, pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD, TargetRPM/2, stopFuncHighCurrent)
	if err != nil {
		return err
	}
	pressure, err = vg.readRobustAveragePressure(ctx, numMeasurements)
	if err != nil {
		return err
	}
	position, err = vg.motor.Position(ctx)
	if err != nil {
		return err
	}
	if pressure > vg.pressureLimit {
		vg.closedPos = position
		vg.closeDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		pressureOpen = pressure
	}

	// Sanity check; if the pressure difference between open & closed position is too small,
	// something went wrong
	// TODO: I think this has to be improved; think more about it
	if math.Abs(float64(pressureOpen-pressureClosed)) < vg.pressureLimit/2 {
		return errors.Errorf("init: pressure same open and closed, something is wrong, positions (closed, open): %f %f, pressures (closed, open): %t %t",
			vg.closedPos, vg.openPos, pressureClosed, pressureOpen)
	}

	if math.Signbit(vg.openPos - vg.closedPos) {
		vg.openPos += OpenPosOffset
	} else {
		vg.openPos -= OpenPosOffset
	}

	return nil
}

// Open opens the jaws.
func (vg *GripperV2) Open(ctx context.Context) error {
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
		isOn, err := vg.motor.IsOn(ctx)
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
	err = vg.motor.GoTo(ctx, TargetRPM, vg.closedPos)
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

		if !isOn {
			return false, nil
		}

		pressure, _, current, err := vg.analogs(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
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
			err = vg.motor.Go(ctx, vg.closeDirection, vg.holdingPressure)
			return true, err
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
			return false, vg.stopAfterError(ctx, errors.Errorf("close timed out, wanted: %f at: %f pressure: %d",
				vg.closedPos, now, pressureRaw))
		}
	}
}

// processCurrentReading checks if the current is within a healthy range or not.
func (vg *GripperV2) processCurrentReading(ctx context.Context, current int, where string) error {
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

// Close stops the motors
func (vg *GripperV2) Close() error {
	return vg.Stop(context.Background())
}

func (vg *GripperV2) stopAfterError(ctx context.Context, other error) error {
	return multierr.Combine(other, vg.motor.Off(ctx))
}

// Stop stops the motors
func (vg *GripperV2) Stop(ctx context.Context) error {
	return vg.motor.Off(ctx)
}

// readCurrent reads the current
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
	fmt.Println(averagePressure)
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
	averagePressure := float64(sum / (len(matrix) * len(matrix[0])))
	return averagePressure, nil
}

// hasPressure checks if the average pressure measurement is above the
// pressureLimit threshold or not.
func (vg *GripperV2) hasPressure(ctx context.Context) (bool, float64, error) {
	p, err := vg.readAveragePressure(ctx)
	return p > float64(vg.pressureLimit), p, err
}

// analogs returns analog measurements such as the average pressure from the ForceMatrix, current, and
// a boolean that indicates if the average pressure is above the pressure limit.
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
