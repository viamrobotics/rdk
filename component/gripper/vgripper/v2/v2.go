// Package vgripper implements versions of the Viam gripper.
package vgripper

import (
	"context"

	// used to import model referenceframe.
	_ "embed"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/forcematrix"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

// modelName is used to register the gripper to a model name.
const modelName = "viam-v2"

//go:embed vgripper_model.json
var vgripperv2json []byte

func init() {
	registry.RegisterComponent(gripper.Subtype, modelName, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return newGripper(ctx, r, config, logger)
		},
	})
}

// Parameters for calibration & operating the gripper.
const (
	maxCurrent              = 3.2
	currentBadReadingCounts = 50
	openPosOffset           = 0.4  // Reduce maximum opening width, keeps out of mechanical binding region
	numMeasurementsCalib    = 10   // Number of measurements at each end position taken when calibrating the gripper
	positionTolerance       = 1    // Tolerance for motor position when reaching the open or closed position
	openTimeout             = 5000 // unit: [ms]
	grabTimeout             = 5000 // unit: [ms]
)

// gripperV2 represents a Viam gripper which operates with a ForceMatrix.
type gripperV2 struct {
	// components of the gripper (board implicitly included)
	motor       motor.GoTillStopSupportingMotor
	current     board.AnalogReader
	forceMatrix forcematrix.ForceMatrix

	// parameters that are set by the user
	startHoldingPressure               float64
	startGripPowerPct                  float64 // percentage of power the gripper uses to hold an item; range: [0-1]
	hasPressureThreshold               float64
	calibrationMinPressureDifference   float64
	antiSlipTimeStepSizeInMilliseconds float64
	antiSlipGripPowerPctStepSize       float64
	targetRPM                          float64
	activatedAntiSlipGripPowerControl  bool

	// action state machine
	state    gripperState
	stateMu  sync.Mutex
	actionMu sync.Mutex

	// determined during the calibration
	openPos, closedPos             float64
	closedDirection, openDirection int64

	// other
	model                 referenceframe.Model
	numBadCurrentReadings int
	logger                golog.Logger
}

// newGripper returns a gripperV2 which operates with a ForceMatrix.
func newGripper(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (*gripperV2, error) {
	boardName := config.Attributes.String("board")
	board, exists := r.BoardByName(boardName)
	if !exists {
		return nil, errors.Errorf("%v gripper requires a board called %v", modelName, boardName)
	}

	motorName := config.Attributes.String("motor")
	_motor, exists := r.MotorByName(motorName)
	if !exists {
		return nil, errors.Errorf("failed to find motor named '%v'", motorName)
	}

	supportedFeatures, err := _motor.GetFeatures(ctx)
	if err != nil {
		return nil, err
	}
	supported, ok := supportedFeatures[motor.PositionReporting]
	if !supported || !ok {
		return nil, errors.New("gripper motor needs to support position")
	}

	stoppableMotor, ok := _motor.(motor.GoTillStopSupportingMotor)
	if !ok {
		return nil, errors.New("gripper motor must support GoTillStop")
	}

	currentAnalogReaderName := config.Attributes.String("current")
	current, exists := board.AnalogReaderByName(currentAnalogReaderName)
	if !exists {
		return nil, errors.Errorf("failed to find analog reader named '%v'", currentAnalogReaderName)
	}

	forceMatrixName := config.Attributes.String("forcematrix")
	forceMatrixDevice, ok := forcematrix.FromRobot(r, forceMatrixName)
	if !ok {
		return nil, errors.Errorf("%q not found or not a force matrix sensor", forceMatrixName)
	}

	hasPressureThreshold := config.Attributes.Float64("has_pressure_threshold", 30)
	calibrationMinPressureDifference := config.Attributes.Float64("calibration_min_pressure_difference", 7)
	activatedAntiSlipGripPowerControl := config.Attributes.Bool("activated_anti_slip_grip_power_control", true)
	startHoldingPressure := config.Attributes.Float64("start_holding_pressure", 0.005)
	startGripPowerPct := config.Attributes.Float64("start_grip_power_pct", 0.005)
	antiSlipTimeStepSizeInMilliseconds := config.Attributes.Float64("anti_slip_time_step_size_in_milliseconds", 10)
	antiSlipGripPowerPctStepSize := config.Attributes.Float64("anti_slip_grip_power_pct_step_size", 0.005)
	targetRPM := config.Attributes.Float64("target_rpm", 200)

	model, err := referenceframe.ParseJSON(vgripperv2json, "")
	if err != nil {
		return nil, err
	}

	vg := &gripperV2{
		// components of the gripper
		motor:       stoppableMotor,
		current:     current,
		forceMatrix: forceMatrixDevice,
		// parameters that are set by the user
		startHoldingPressure:               startHoldingPressure,
		startGripPowerPct:                  startGripPowerPct,
		hasPressureThreshold:               hasPressureThreshold,
		calibrationMinPressureDifference:   calibrationMinPressureDifference,
		antiSlipTimeStepSizeInMilliseconds: antiSlipTimeStepSizeInMilliseconds,
		antiSlipGripPowerPctStepSize:       antiSlipGripPowerPctStepSize,
		targetRPM:                          targetRPM,
		activatedAntiSlipGripPowerControl:  activatedAntiSlipGripPowerControl,
		// action state machine
		state: gripperStateUnspecified,
		// other
		logger: logger,
		model:  model,
	}

	if err := vg.Calibrate(ctx); err != nil {
		return nil, err
	}

	if err := vg.Open(ctx); err != nil {
		return nil, err
	}

	return vg, nil
}

// Idle sets the state to a passive state that is neither grabbing nor opening.
func (vg *gripperV2) Idle() {
	vg.stateMu.Lock()
	defer vg.stateMu.Unlock()
	vg.state = gripperStateIdle
}

// State returns the state of the gripper.
func (vg *gripperV2) State() gripperState {
	vg.stateMu.Lock()
	defer vg.stateMu.Unlock()
	return vg.state
}

// Calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *gripperV2) Calibrate(ctx context.Context) error {
	vg.stateMu.Lock()
	vg.state = gripperStateCalibrating
	vg.stateMu.Unlock()
	vg.actionMu.Lock()
	defer vg.actionMu.Unlock()
	defer vg.Idle()
	return vg.calibrate(ctx)
}

// Open opens the jaws.
func (vg *gripperV2) Open(ctx context.Context) error {
	vg.stateMu.Lock()
	vg.state = gripperStateOpening
	vg.stateMu.Unlock()
	vg.actionMu.Lock()
	defer vg.actionMu.Unlock()
	return vg.open(ctx)
}

// Grab closes the jaws until pressure is sensed and returns true,
// or until closed position is reached, and returns false.
func (vg *gripperV2) Grab(ctx context.Context) (bool, error) {
	vg.stateMu.Lock()
	vg.state = gripperStateGrabbing
	vg.stateMu.Unlock()

	vg.actionMu.Lock()
	grabbingSuccess, err := vg.grab(ctx)
	vg.actionMu.Unlock()
	if err != nil {
		return grabbingSuccess, err
	}

	if grabbingSuccess && vg.activatedAntiSlipGripPowerControl {
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
	vg.activatedAntiSlipGripPowerControl = activate
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
	case gripperStateUnspecified:
		defer vg.stateMu.Unlock()
		return errors.New("gripper state is unspecified")
	case gripperStateGrabbing:
		// TODO: Make sure this case is ok. We might want to
		// make sure we've grabbed something successfully before going straight into AntiSlipForceControlling
		vg.state = gripperStateAntiSlipForceControlling
	case gripperStateAntiSlipForceControlling:
	case gripperStateCalibrating, gripperStateIdle, gripperStateOpening:
		fallthrough
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
	msPer := vg.antiSlipTimeStepSizeInMilliseconds
	gripPowerPct := vg.startGripPowerPct
	for {
		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
			return vg.stopAfterError(ctx, ctx.Err())
		}

		if !vg.activatedAntiSlipGripPowerControl {
			return nil
		}
		switch vg.State() {
		case gripperStateUnspecified:
			return errors.New("gripper state is unspecified")
		case gripperStateAntiSlipForceControlling:
		case gripperStateCalibrating, gripperStateGrabbing, gripperStateIdle, gripperStateOpening:
			fallthrough
		default:
			return nil
		}
		// vg.logger.Debugf("antiSlipForceControl, state: %v", vg.state.String())

		// Adjust grip strength
		objectDetectSlip, err := vg.forceMatrix.DetectSlip(ctx)
		if err != nil {
			return err
		}
		if objectDetectSlip {
			gripPowerPct += vg.antiSlipGripPowerPctStepSize
			err = vg.motor.SetPower(ctx, float64(vg.closedDirection)*gripPowerPct)
			if err != nil {
				return err
			}
		}

		_, _, current, err := vg.analogs(ctx)
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		err = vg.checkCurrentInAcceptableRange(current, "anti-slip force control")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}
	}
}

// calibrate finds the open and close position, as well as which motor direction
// corresponds to opening and closing the gripper.
func (vg *gripperV2) calibrate(ctx context.Context) error {
	// This will be passed to GoTillStop
	stopFuncHighCurrent := func(ctx context.Context) bool {
		current, err := vg.readCurrent(ctx)
		if err != nil {
			vg.logger.Error(err)
			return true
		}

		err = vg.checkCurrentInAcceptableRange(current, "init")
		if err != nil {
			vg.logger.Error(err)
			return true
		}
		return false
	}

	// Test forward motion for pressure/endpoint
	vg.logger.Debug("init: moving forward")
	err := vg.motor.GoTillStop(ctx, vg.targetRPM/2, stopFuncHighCurrent)
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
	if pressure > vg.hasPressureThreshold {
		vg.closedPos = position
		vg.closedDirection = 1
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = 1
		pressureOpen = pressure
	}

	// Test backward motion for pressure/endpoint
	vg.logger.Debug("init: moving backward")
	err = vg.motor.GoTillStop(ctx, -1*vg.targetRPM/2, stopFuncHighCurrent)
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
	if pressure > vg.hasPressureThreshold {
		vg.closedPos = position
		vg.closedDirection = -1
		pressureClosed = pressure
	} else {
		vg.openPos = position
		vg.openDirection = -1
		pressureOpen = pressure
	}

	// Sanity check; if the pressure difference between open & closed position is too small,
	// something went wrong
	if math.Abs(pressureOpen-pressureClosed) < vg.calibrationMinPressureDifference ||
		vg.closedDirection == 0 ||
		vg.openDirection == 0 {
		return errors.Errorf("init: open and closed positions can't be distinguished: "+
			"positions (closed, open): %f %f, pressures (closed, open): %f %f, "+
			"open direction: %v, closed direction: %v",
			vg.closedPos, vg.openPos, pressureClosed, pressureOpen, vg.openDirection, vg.closedDirection)
	}

	if vg.openDirection == vg.closedDirection {
		return errors.New("openDirection and vg.closedDirection have to be opposed")
	}

	vg.logger.Debugf("init: orig openPos: %f, closedPos: %f", vg.openPos, vg.closedPos)
	vg.openPos += math.Copysign(openPosOffset, (vg.closedPos - vg.openPos))
	vg.logger.Debugf("init: offset openPos: %f, closedPos: %f", vg.openPos, vg.closedPos)

	// Zero to closed position
	curPos, err := vg.motor.Position(ctx)
	if err != nil {
		return err
	}
	err = vg.motor.ResetZeroPosition(ctx, curPos-vg.closedPos)
	if err != nil {
		return err
	}
	vg.openPos -= vg.closedPos
	vg.closedPos = 0

	vg.logger.Debugf("init: final openPos: %f, closedPos: %f", vg.openPos, vg.closedPos)

	vg.state = gripperStateIdle
	return nil
}

// ModelFrame returns the dynamic frame of the model.
func (vg *gripperV2) ModelFrame() referenceframe.Model {
	return vg.model
}

// open opens the jaws.
func (vg *gripperV2) open(ctx context.Context) error {
	vg.logger.Debugf("In open fcn: %v", vg.state.String())
	switch vg.State() {
	case gripperStateUnspecified:
		return errors.New("gripper state is unspecified")
	case gripperStateOpening:
	case gripperStateAntiSlipForceControlling, gripperStateCalibrating, gripperStateGrabbing, gripperStateIdle:
		fallthrough
	default:
		return errors.New("gripper state is unspecified")
	}
	err := vg.Stop(ctx)
	if err != nil {
		return err
	}
	err = vg.motor.GoTo(ctx, vg.targetRPM, vg.openPos)
	if err != nil {
		return err
	}

	msPer := 10
	total := 0
	for {
		// vg.logger.Debugf("Opening, state: %v", vg.state.String())

		switch vg.State() {
		case gripperStateUnspecified:
			return errors.New("gripper state is unspecified")
		case gripperStateOpening:
		case gripperStateAntiSlipForceControlling, gripperStateCalibrating, gripperStateGrabbing, gripperStateIdle:
			fallthrough
		default:
			return nil
		}

		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
			return vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to open
		isOn, err := vg.motor.IsPowered(ctx)
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
		err = vg.checkCurrentInAcceptableRange(current, "opening")
		if err != nil {
			return vg.stopAfterError(ctx, err)
		}

		total += msPer
		if total > openTimeout {
			measuredPos, err := vg.motor.Position(ctx)
			return vg.stopAfterError(
				ctx,
				multierr.Combine(errors.Errorf("open timed out, wanted: %f at: %f", vg.openPos, measuredPos), err),
			)
		}
	}
}

// grab closes the jaws until pressure is sensed and returns true,
// or until closed position is reached, and returns false.
func (vg *gripperV2) grab(ctx context.Context) (bool, error) {
	vg.logger.Debugf("In grab fcn: %v", vg.state.String())
	switch vg.State() {
	case gripperStateUnspecified:
		return false, errors.New("gripper state is unspecified")
	case gripperStateGrabbing:
	case gripperStateAntiSlipForceControlling, gripperStateCalibrating, gripperStateIdle, gripperStateOpening:
		fallthrough
	default:
		return false, nil
	}
	err := vg.Stop(ctx)
	if err != nil {
		return false, err
	}
	err = vg.motor.GoTo(ctx, vg.targetRPM, vg.closedPos)
	if err != nil {
		return false, err
	}

	msPer := 10
	total := 0
	for {
		// vg.logger.Debugf("Grabbing, state: %v", vg.state.String())
		switch vg.State() {
		case gripperStateUnspecified:
			return false, errors.New("gripper state is unspecified")
		case gripperStateGrabbing:
		case gripperStateAntiSlipForceControlling, gripperStateCalibrating, gripperStateIdle, gripperStateOpening:
			fallthrough
		default:
			return false, nil
		}

		wait := utils.SelectContextOrWait(ctx, time.Duration(msPer)*time.Millisecond)
		if !wait {
			return false, vg.stopAfterError(ctx, ctx.Err())
		}
		// If motor went all the way to closed
		isOn, err := vg.motor.IsPowered(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		hasPressure, pressure, current, err := vg.analogs(ctx)
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		if !isOn {
			measuredPos, err := vg.motor.Position(ctx)
			if err != nil {
				return false, err
			}
			if !hasPressure && math.Abs(measuredPos-vg.closedPos) > positionTolerance {
				return false, errors.Errorf("didn't reach closed position and am not holding an object,"+
					"closed position: %f +/- %v tolerance, actual position: %f", vg.closedPos, positionTolerance, measuredPos)
			}
			return false, nil
		}

		err = vg.checkCurrentInAcceptableRange(current, "grabbing")
		if err != nil {
			return false, vg.stopAfterError(ctx, err)
		}

		if pressure >= vg.startHoldingPressure {
			now, err := vg.motor.Position(ctx)
			if err != nil {
				return false, err
			}
			vg.logger.Debugf("i think i grabbed something, have pressure, pos: %f closedPos: %v", now, vg.closedPos)
			err = vg.motor.SetPower(ctx, float64(vg.closedDirection)*vg.startGripPowerPct)
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
func (vg *gripperV2) checkCurrentInAcceptableRange(current float64, where string) error {
	// vg.logger.Debugf("Motor Current: %f", current)
	if math.Abs(current) < maxCurrent {
		vg.numBadCurrentReadings = 0
		return nil
	}
	vg.numBadCurrentReadings++
	if vg.numBadCurrentReadings < currentBadReadingCounts {
		return nil
	}
	return errors.Errorf("current too high for too long, currently %f during %s", current, where)
}

// Close stops the motors.
func (vg *gripperV2) Close(ctx context.Context) error {
	return vg.Stop(ctx)
}

// stopAfterError stops the motor and returns the combined errors.
func (vg *gripperV2) stopAfterError(ctx context.Context, other error) error {
	return multierr.Combine(other, vg.motor.Stop(ctx))
}

// Stop stops the motors.
func (vg *gripperV2) Stop(ctx context.Context) error {
	return vg.motor.Stop(ctx)
}

// readCurrent reads the current and returns signed (bidirectional) amperage.
func (vg *gripperV2) readCurrent(ctx context.Context) (float64, error) {
	raw, err := vg.current.Read(ctx)
	if err != nil {
		return 0, err
	}

	// 3.3v / 10-bit adc, raw adc value, -1.65 offset to center of 3.3v, 200mV/A current sensor
	// ACTUAL grippers have magnetic interference from the motor in the current (Dec. 2021) iteration
	// Offset is set to -1.12 (instead of -1.65) to compensate
	current := (((3.3 / 1023) * float64(raw)) - 1.12) / 0.2

	return current, nil
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
	matrix, err := vg.forceMatrix.ReadMatrix(ctx)
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
// hasPressureThreshold threshold or not.
func (vg *gripperV2) hasPressure(ctx context.Context) (bool, float64, error) {
	p, err := vg.readAveragePressure(ctx)
	if err != nil {
		return false, 0, err
	}
	return p > vg.hasPressureThreshold, p, err
}

// analogs returns measurements such as: boolean that indicates if the average
// pressure is above the pressure limit, the average pressure from the ForceMatrix,
// and the current in the motor.
func (vg *gripperV2) analogs(ctx context.Context) (bool, float64, float64, error) {
	hasPressure, pressure, errP := vg.hasPressure(ctx)
	current, errC := vg.readCurrent(ctx)
	err := multierr.Combine(errP, errC)
	if err != nil {
		return false, 0, 0, err
	}
	return hasPressure, pressure, current, nil
}
