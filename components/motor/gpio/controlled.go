package gpio

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

const getPID = "get_tuned_pid"

// SetState sets the state of the motor for the built-in control loop.
func (cm *controlledMotor) SetState(ctx context.Context, state []*control.Signal) error {
	if cm.loop != nil && !cm.loop.Running() {
		return nil
	}
	power := state[0].GetSignalValueAt(0)
	return cm.real.SetPower(ctx, power, nil)
}

// State gets the state of the motor for the built-in control loop.
func (cm *controlledMotor) State(ctx context.Context) ([]float64, error) {
	ticks, _, err := cm.enc.Position(ctx, encoder.PositionTypeTicks, nil)
	pos := ticks + cm.offsetInTicks
	return []float64{pos}, err
}

// updateControlBlockPosVel updates the trap profile and the constant set point for position and velocity control.
func (cm *controlledMotor) updateControlBlock(ctx context.Context, setPoint, maxVel float64) error {
	// Update the Trapezoidal Velocity Profile block with the given maxVel for velocity control
	dependsOn := []string{cm.blockNames[control.BlockNameConstant][0], cm.blockNames[control.BlockNameEndpoint][0]}
	if err := control.UpdateTrapzBlock(ctx, cm.blockNames[control.BlockNameTrapezoidal][0], maxVel, dependsOn, cm.loop); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	if err := control.UpdateConstantBlock(ctx, cm.blockNames[control.BlockNameConstant][0], setPoint, cm.loop); err != nil {
		return err
	}

	return nil
}

func (cm *controlledMotor) setupControlLoop(conf *Config) error {
	// set the necessary options for an encoded motor
	options := control.Options{
		PositionControlUsingTrapz: true,
		LoopFrequency:             100.0,
	}

	// convert the motor config ControlParameters to the control.PIDConfig structure for use in setup_control.go
	cm.configPIDVals = []control.PIDConfig{{
		Type: "",
		P:    conf.ControlParameters.P,
		I:    conf.ControlParameters.I,
		D:    conf.ControlParameters.D,
	}}

	// auto tune motor if all ControlParameters are 0
	// since there's only one set of PID values for a motor, they will always be at convertedControlParams[0]
	if cm.configPIDVals[0].NeedsAutoTuning() {
		options.NeedsAutoTuning = true
	}

	pl, err := control.SetupPIDControlConfig(cm.configPIDVals, cm.Name().ShortName(), options, cm, cm.logger)
	if err != nil {
		return err
	}

	cm.controlLoopConfig = *pl.ControlConf
	cm.loop = pl.ControlLoop
	cm.blockNames = pl.BlockNames
	cm.tunedVals = pl.TunedVals

	return nil
}

func (cm *controlledMotor) startControlLoop() error {
	loop, err := control.NewLoop(cm.logger, cm.controlLoopConfig, cm)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	cm.loop = loop

	return nil
}

func setupMotorWithControls(
	_ context.Context,
	m *Motor,
	enc encoder.Encoder,
	cfg resource.Config,
	logger logging.Logger,
) (motor.Motor, error) {
	conf, err := resource.NativeConfig[*Config](cfg)
	if err != nil {
		return nil, err
	}

	tpr := float64(conf.TicksPerRotation)
	if tpr == 0 {
		tpr = 1.0
	}

	cm := &controlledMotor{
		Named:            cfg.ResourceName().AsNamed(),
		logger:           logger,
		opMgr:            operation.NewSingleOperationManager(),
		tunedVals:        &[]control.PIDConfig{{}},
		ticksPerRotation: tpr,
		real:             m,
		enc:              enc,
	}

	// setup control loop
	if conf.ControlParameters == nil {
		return nil, motor.NewControlParametersUnimplementedError()
	}
	if err := cm.setupControlLoop(conf); err != nil {
		return nil, err
	}

	return cm, nil
}

type controlledMotor struct {
	resource.Named
	resource.AlwaysRebuild
	logger                  logging.Logger
	opMgr                   *operation.SingleOperationManager
	activeBackgroundWorkers sync.WaitGroup

	offsetInTicks    float64
	ticksPerRotation float64

	mu   sync.RWMutex
	real *Motor
	enc  encoder.Encoder

	controlLoopConfig control.Config
	blockNames        map[string][]string
	loop              *control.Loop
	configPIDVals     []control.PIDConfig
	tunedVals         *[]control.PIDConfig
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational.
func (cm *controlledMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	cm.opMgr.CancelRunning(ctx)
	if cm.loop != nil {
		cm.loop.Pause()
	}
	return cm.real.SetPower(ctx, powerPct, nil)
}

// IsPowered returns whether or not the motor is currently on, and the percent power (between 0
// and 1, if the motor is off then the percent power will be 0).
func (cm *controlledMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return cm.real.IsPowered(ctx, extra)
}

// IsMoving returns if the motor is moving or not.
func (cm *controlledMotor) IsMoving(ctx context.Context) (bool, error) {
	return cm.real.IsMoving(ctx)
}

// Stop stops rpmMonitor and stops the real motor.
func (cm *controlledMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	// after the motor is created, Stop is called, but if the PID controller
	// is auto-tuning, the loop needs to keep running
	if cm.loop != nil && !cm.loop.GetTuning(ctx) {
		cm.loop.Pause()

		// update pid controller to use the current state as the desired state
		currentTicks, _, err := cm.enc.Position(ctx, encoder.PositionTypeTicks, extra)
		if err != nil {
			return err
		}
		if err := cm.updateControlBlock(ctx, currentTicks+cm.offsetInTicks, cm.real.maxRPM*cm.ticksPerRotation/60); err != nil {
			return err
		}
	}
	return cm.real.Stop(ctx, nil)
}

// Close cleanly shuts down the motor.
func (cm *controlledMotor) Close(ctx context.Context) error {
	if err := cm.Stop(ctx, nil); err != nil {
		return err
	}
	if cm.loop != nil {
		cm.loop.Stop()
		cm.loop = nil
	}
	cm.activeBackgroundWorkers.Wait()
	return nil
}

// Properties returns whether or not the motor supports certain optional properties.
func (cm *controlledMotor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
	}, nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (cm *controlledMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, _, err := cm.enc.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return 0, err
	}

	// offsetTicks in Rotation can be changed by ResetPosition
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return (ticks + cm.offsetInTicks) / cm.ticksPerRotation, nil
}

// ResetZeroPosition sets the current position (+/- offset) to be the new zero (home) position.
func (cm *controlledMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	// Stop the motor if resetting position
	if err := cm.Stop(ctx, extra); err != nil {
		return err
	}
	if err := cm.enc.ResetPosition(ctx, extra); err != nil {
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.offsetInTicks = -1 * offset * cm.ticksPerRotation
	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position
// This will block until the position has been reached.
func (cm *controlledMotor) GoTo(ctx context.Context, rpm, targetPosition float64, extra map[string]interface{}) error {
	// no op manager added, we're relying on GoFor's oepration manager in this driver
	pos, err := cm.Position(ctx, extra)
	if err != nil {
		return err
	}
	rotations := targetPosition - pos

	// ignore the direction of rpm
	rpm = math.Abs(rpm)

	// if you call GoFor with 0 revolutions, the motor will spin forever. If we are at the target,
	// we must avoid this by not calling GoFor.
	if rdkutils.Float64AlmostEqual(rotations, 0, 0.1) {
		cm.logger.CDebug(ctx, "GoTo distance nearly zero, not moving")
		return nil
	}
	return cm.GoFor(ctx, rpm, rotations, extra)
}

// SetRPM instructs the motor to move at the specified RPM indefinitely.
func (cm *controlledMotor) SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error {
	cm.opMgr.CancelRunning(ctx)
	ctx, done := cm.opMgr.New(ctx)
	defer done()

	warning, err := motor.CheckSpeed(rpm, cm.real.maxRPM)
	if warning != "" {
		cm.logger.CWarn(ctx, warning)
	}
	if err != nil {
		return err
	}

	if err := cm.checkTuningStatus(); err != nil {
		return err
	}

	if cm.loop == nil {
		// create new control loop
		if err := cm.startControlLoop(); err != nil {
			return err
		}
	}

	// set control loop values
	velVal := math.Abs(rpm * cm.ticksPerRotation / 60)
	goalPos := math.Inf(int(rpm))
	// setPoint is +/- infinity, maxVel is calculated velVal
	if err := cm.updateControlBlock(ctx, goalPos, velVal); err != nil {
		return err
	}
	cm.loop.Resume()

	return nil
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are
// negative the motor will spin in the forward direction.
// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
func (cm *controlledMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	cm.opMgr.CancelRunning(ctx)
	ctx, done := cm.opMgr.New(ctx)
	defer done()

	warning, err := motor.CheckSpeed(rpm, cm.real.maxRPM)
	if warning != "" {
		cm.logger.CWarn(ctx, warning)
	}
	if err != nil {
		return err
	}

	if err := motor.CheckRevolutions(revolutions); err != nil {
		return err
	}

	currentTicks, _, err := cm.enc.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return err
	}

	if err := cm.checkTuningStatus(); err != nil {
		return err
	}

	if cm.loop == nil {
		// create new control loop
		if err := cm.startControlLoop(); err != nil {
			return err
		}
	}

	goalPos, _, _ := encodedGoForMath(rpm, revolutions, currentTicks+cm.offsetInTicks, cm.ticksPerRotation)

	// set control loop values
	velVal := math.Abs(rpm * cm.ticksPerRotation / 60)
	// when rev = 0, only velocity is controlled
	// setPoint is +/- infinity, maxVel is calculated velVal
	if err := cm.updateControlBlock(ctx, goalPos, velVal); err != nil {
		return err
	}
	cm.loop.Resume()

	// we can probably use something in controls to make GoFor blockign without this
	// helper function
	positionReached := func(ctx context.Context) (bool, error) {
		var errs error
		pos, _, posErr := cm.enc.Position(ctx, encoder.PositionTypeTicks, extra)
		errs = multierr.Combine(errs, posErr)
		if rdkutils.Float64AlmostEqual(pos+cm.offsetInTicks, goalPos, 2.0) {
			stopErr := cm.Stop(ctx, extra)
			errs = multierr.Combine(errs, stopErr)
			return true, errs
		}
		return false, errs
	}
	err = cm.opMgr.WaitForSuccess(
		ctx,
		10*time.Millisecond,
		positionReached,
	)
	// Ignore the context canceled error - this occurs when the motor is stopped
	// at the beginning of goForInternal
	if !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

func (cm *controlledMotor) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	resp := make(map[string]interface{})

	cm.mu.Lock()
	defer cm.mu.Unlock()
	ok := req[getPID].(bool)
	if ok {
		var respStr string
		if !(*cm.tunedVals)[0].NeedsAutoTuning() {
			respStr += fmt.Sprintf("{p: %v, i: %v, d: %v, type: %v} ",
				(*cm.tunedVals)[0].P, (*cm.tunedVals)[0].I, (*cm.tunedVals)[0].D, (*cm.tunedVals)[0].Type)
		}
		resp[getPID] = respStr
	}

	return resp, nil
}

// if loop is tuning, return an error
// if loop has been tuned but the values haven't been added to the config, error with tuned values.
func (cm *controlledMotor) checkTuningStatus() error {
	if cm.loop != nil && cm.loop.GetTuning(context.Background()) {
		return control.TuningInProgressErr(cm.Name().ShortName())
	} else if cm.configPIDVals[0].NeedsAutoTuning() && !(*cm.tunedVals)[0].NeedsAutoTuning() {
		return control.TunedPIDErr(cm.Name().ShortName(), *cm.tunedVals)
	}
	return nil
}
