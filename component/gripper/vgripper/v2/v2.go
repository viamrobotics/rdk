// Package vgripper implements versions of the Viam gripper.
package vgripper

import (
	"context"
	_ "embed" // used to import model frame
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/component/gripper"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor/forcematrix"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// modelName is used to register the gripper to a model name.
const modelName = "viam-v2"

//go:embed vgripper_model.json
var vgripperv2json []byte

func init() {
	registry.RegisterComponent(gripper.Subtype, modelName, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return new(ctx, r, config, logger)
		},
	})
}

// Parameters for calibration & operating the gripper.
const (
	targetRPM               = 200
	maxCurrent              = 300
	currentBadReadingCounts = 50
	openPosOffset           = 0.4   // Reduce maximum opening width, keeps out of mechanical binding region
	numMeasurementsCalib    = 10    // Number of measurements at each end position taken when calibrating the gripper
	positionTolerance       = 1     // Tolerance for motor position when reaching the open or closed position
	openTimeout             = 5000  // unit: [ms]
	grabTimeout             = 5000  // unit: [ms]
	holdingPressure         = 0.005 //  percentage of power the gripper uses to hold an item; range: [0-1]
)

// gripperV2 represents a Viam gripper which operates with a ForceMatrix.
type gripperV2 struct {
	// components of the gripper (board implicitly included)
	motor       motor.Motor
	current     board.AnalogReader
	forceMatrix forcematrix.ForceMatrix

	// parameters that are set by the user
	holdingPressure               float32
	pressureLimit                 float64
	calibrationNoiseThreshold     float64
	activatedAntiSlipForceControl bool // flag

	// action state machine
	state    gripperState
	stateMu  *sync.Mutex
	actionMu *sync.Mutex

	// determined during the calibration
	openPos, closedPos             float64
	closedDirection, openDirection pb.DirectionRelative

	// other
	model                 *referenceframe.Model
	numBadCurrentReadings int
	logger                golog.Logger
}

// new returns a gripperV2 which operates with a ForceMatrix.
func new(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*gripperV2, error) {
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

	model, err := referenceframe.ParseJSON(vgripperv2json, "")
	if err != nil {
		return nil, err
	}

	vg := &gripperV2{
		// components of the gripper
		motor:       motor,
		current:     current,
		forceMatrix: forceMatrixDevice,
		// parameters that are set by the user
		holdingPressure:               holdingPressure,
		pressureLimit:                 pressureLimit,
		calibrationNoiseThreshold:     calibrationNoiseThreshold,
		activatedAntiSlipForceControl: true,
		// action state machine
		state:    gripperState_UNSPECIFIED,
		stateMu:  &sync.Mutex{},
		actionMu: &sync.Mutex{},
		// switchActionChannel: make(chan bool),
		// other
		logger: logger,
		model:  model,
	}

	if err := vg.Calibrate(ctx, logger); err != nil {
		return nil, err
	}

	if err := vg.Open(ctx); err != nil {
		return nil, err
	}

	return vg, nil
}

func (vg *gripperV2) Idle() {
	vg.stateMu.Lock()
	vg.state = gripperState_IDLE
	defer vg.stateMu.Unlock()
}

func (vg *gripperV2) State() gripperState {
	vg.stateMu.Lock()
	defer vg.stateMu.Unlock()
	return vg.state
}

// Calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *gripperV2) Calibrate(ctx context.Context, logger golog.Logger) error {
	vg.stateMu.Lock()
	vg.state = gripperState_CALIBRATING
	vg.stateMu.Unlock()
	vg.actionMu.Lock()
	defer vg.actionMu.Unlock()
	defer vg.Idle()
	return vg.calibrate(ctx, logger)
}

// Open opens the jaws.
func (vg *gripperV2) Open(ctx context.Context) error {
	vg.stateMu.Lock()
	vg.state = gripperState_OPENING
	vg.stateMu.Unlock()
	vg.actionMu.Lock()
	defer vg.actionMu.Unlock()
	return vg.open(ctx)
}

// Grab closes the jaws until pressure is sensed and returns true,
// or until closed position is reached, and returns false.
func (vg *gripperV2) Grab(ctx context.Context) (bool, error) {
	vg.stateMu.Lock()
	vg.state = gripperState_GRABBING
	vg.stateMu.Unlock()

	vg.actionMu.Lock()
	grabbingSuccess, err := vg.grab(ctx)
	vg.actionMu.Unlock()
	if err != nil {
		return grabbingSuccess, err
	}

	if grabbingSuccess && vg.activatedAntiSlipForceControl {
		err = vg.AntiSlipForceControl(ctx)
		if err != nil {
			return false, err
		}
	}
	return grabbingSuccess, err
}

// ActivateAntiSlipForceControl sets the flag that determines whether or not the gripper
// adjusts the holding pressure based on the presence of slip; or not.
func (vg *gripperV2) ActivateAntiSlipForceControl(ctx context.Context, activate bool) {
	vg.activatedAntiSlipForceControl = activate
}

// AntiSlipForceControl controls adaptively the pressure while holding an object
// with the aim to prevent it from slipping.
func (vg *gripperV2) AntiSlipForceControl(ctx context.Context) error {
	vg.stateMu.Lock()

	// Start AntiSlipForceControl only if we're still in the grabbing state,
	// or already in the anti-slip force controlling state.
	// If we are in a specific state that's defined, someone probably decided
	// to go for another action than staying in AntiSlipForceControl.
	switch vg.state {
	case gripperState_UNSPECIFIED:
		defer vg.stateMu.Unlock()
		return errors.New("gripper state is unspecified")
	case gripperState_GRABBING:
		// TODO: Make sure this case is ok. We might want to
		// make sure we've grabbed something successfully before going straight into AntiSlipForceControlling
		vg.state = gripperState_ANTI_SLIP_FORCE_CONTROLLING
	case gripperState_ANTI_SLIP_FORCE_CONTROLLING:
	default:
		defer vg.stateMu.Unlock()
		return nil
	}

	vg.stateMu.Unlock()
	vg.actionMu.Lock()
	defer vg.actionMu.Unlock()
	return vg.antiSlipForceControl(ctx)
}

// antiSlipForceControl controls adaptively the pressure while holding an object
// with the aim to prevent it from slipping.
func (vg *gripperV2) antiSlipForceControl(ctx context.Context) error {
	msPer := 10
	antiSlipHoldingPressure := vg.holdingPressure
	for {
		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
			return vg.stopAfterError(ctx, ctx.Err())
		}

		if !vg.activatedAntiSlipForceControl {
			return nil
		}
		switch vg.State() {
		case gripperState_UNSPECIFIED:
			return errors.New("gripper state is unspecified")
		case gripperState_ANTI_SLIP_FORCE_CONTROLLING:
		default:
			return nil
		}
		fmt.Println("antiSlipForceControl, state: ", gripperState_name[vg.state])

		// Adjust grip strength
		objectIsSlipping, err := vg.forceMatrix.IsSlipping(ctx)
		if err != nil {
			return err
		}
		if objectIsSlipping {
			antiSlipHoldingPressure += 0.005
			err = vg.motor.Go(ctx, vg.closedDirection, antiSlipHoldingPressure)
			if err != nil {
				return err
			}
		}

		_, _, current, err := vg.analogs(ctx)
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		err = vg.checkCurrentInAcceptableRange(ctx, current, "anti-slip force control")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}
	}

}

// calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *gripperV2) calibrate(ctx context.Context, logger golog.Logger) error {
	// This will be passed to GoTillStop
	stopFuncHighCurrent := func(ctx context.Context) bool {
		current, err := vg.readCurrent(ctx)
		if err != nil {
			logger.Error(err)
			return true
		}

		err = vg.checkCurrentInAcceptableRange(ctx, current, "init")
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

	vg.openPos += math.Copysign(openPosOffset, (vg.closedPos - vg.openPos))

	vg.state = gripperState_IDLE
	return nil
}

// ModelFrame returns the dynamic frame of the model
func (vg *gripperV2) ModelFrame() *referenceframe.Model {
	return vg.model
}

// open opens the jaws.
func (vg *gripperV2) open(ctx context.Context) error {
	fmt.Println("In open fcn: ", gripperState_name[vg.state])
	switch vg.State() {
	case gripperState_UNSPECIFIED:
		return errors.New("gripper state is unspecified")
	case gripperState_OPENING:
	default:
		return nil
	}
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
		fmt.Println("Opening, state: ", gripperState_name[vg.state])

		switch vg.State() {
		case gripperState_UNSPECIFIED:
			return errors.New("gripper state is unspecified")
		case gripperState_OPENING:
		default:
			return nil
		}

		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
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
		err = vg.checkCurrentInAcceptableRange(ctx, current, "opening")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		total += msPer
		if total > openTimeout {
			measuredPos, err := vg.motor.Position(ctx)
			return vg.stopAfterError(ctx, multierr.Combine(errors.Errorf("open timed out, wanted: %f at: %f", vg.openPos, measuredPos), err))
		}
	}
}

// grab closes the jaws until pressure is sensed and returns true,
// or until closed position is reached, and returns false.
func (vg *gripperV2) grab(ctx context.Context) (bool, error) {
	fmt.Println("In grab fcn: ", gripperState_name[vg.state])
	switch vg.State() {
	case gripperState_UNSPECIFIED:
		return false, errors.New("gripper state is unspecified")
	case gripperState_GRABBING:
	default:
		return false, nil
	}
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
		fmt.Println("Grabbing, state: ", gripperState_name[vg.state])
		switch vg.State() {
		case gripperState_UNSPECIFIED:
			return false, errors.New("gripper state is unspecified")
		case gripperState_GRABBING:
		default:
			return false, nil
		}

		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
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

		err = vg.checkCurrentInAcceptableRange(ctx, current, "grabbing")
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
		if total > grabTimeout {
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

// checkCurrentInAcceptableRange checks if the current is within a healthy range or not.
func (vg *gripperV2) checkCurrentInAcceptableRange(ctx context.Context, current int, where string) error {
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
func (vg *gripperV2) Close() error {
	return vg.Stop(context.Background())
}

// stopAfterError stops the motor and returns the combined errors.
func (vg *gripperV2) stopAfterError(ctx context.Context, other error) error {
	return multierr.Combine(other, vg.motor.Off(ctx))
}

// Stop stops the motors.
func (vg *gripperV2) Stop(ctx context.Context) error {
	return vg.motor.Off(ctx)
}

// readCurrent reads the current.
func (vg *gripperV2) readCurrent(ctx context.Context) (int, error) {
	return vg.current.Read(ctx)
}

// readRobustAveragePressure reads the pressure multiple times and returns the average over
// all matrix cells and number of measurements.
func (vg *gripperV2) readRobustAveragePressure(ctx context.Context, numMeasurements int) (float64, error) {
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
func (vg *gripperV2) readAveragePressure(ctx context.Context) (float64, error) {
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
func (vg *gripperV2) hasPressure(ctx context.Context) (bool, float64, error) {
	p, err := vg.readAveragePressure(ctx)
	if err != nil {
		return false, 0, err
	}
	return p > vg.pressureLimit, p, err
}

// analogs returns measurements such as: boolean that indicates if the average
// pressure is above the pressure limit, the average pressure from the ForceMatrix,
// and the current in the motor.
func (vg *gripperV2) analogs(ctx context.Context) (bool, float64, int, error) {
	hasPressure, pressure, errP := vg.hasPressure(ctx)
	current, errC := vg.readCurrent(ctx)
	err := multierr.Combine(errP, errC)
	if err != nil {
		return false, 0, 0, err
	}
	return hasPressure, pressure, current, nil
}
