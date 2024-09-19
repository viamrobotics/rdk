package sensorcontrolled

import (
	"context"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/control"
)

// SetVelocity commands a base to move at the requested linear and angular velocites.
// When controls are enabled, SetVelocity polls the provided velocity movement sensor and corrects
// any error between the desired velocity and the actual velocity using a PID control loop.
func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	if sb.controlLoopConfig == nil {
		sb.logger.CWarnf(ctx, "control parameters not configured, using %v's SetVelocity method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
	}

	// check tuning status
	if err := sb.checkTuningStatus(); err != nil {
		return err
	}

	// make sure the control loop is enabled
	if sb.loop == nil {
		if err := sb.startControlLoop(); err != nil {
			return err
		}
	}

	// convert linear.Y mmPerSec to mPerSec, angular.Z is degPerSec
	if err := sb.updateControlConfig(ctx, linear.Y/1000.0, angular.Z); err != nil {
		return err
	}
	sb.loop.Resume()

	return nil
}

// startControlLoop uses the control config to initialize a control loop and store it on the sensor controlled base struct.
// The sensor base is the controllable interface that implements State and GetState called from the endpoint block of the control loop.
func (sb *sensorBase) startControlLoop() error {
	loop, err := control.NewLoop(sb.logger, *sb.controlLoopConfig, sb)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	sb.loop = loop

	return nil
}

func (sb *sensorBase) setupControlLoop(linear, angular control.PIDConfig) error {
	// set the necessary options for a sensorcontrolled base
	options := control.Options{
		SensorFeedback2DVelocityControl: true,
		LoopFrequency:                   sb.controlFreq,
		ControllableType:                "base_name",
	}

	// check if either linear or angular need to be tuned
	if linear.NeedsAutoTuning() || angular.NeedsAutoTuning() {
		options.NeedsAutoTuning = true
	}

	// combine linear and angular back into one control.PIDConfig, with linear first
	pidVals := []control.PIDConfig{linear, angular}

	// fully set up the control config based on the provided options
	pl, err := control.SetupPIDControlConfig(pidVals, sb.Name().ShortName(), options, sb, sb.logger)
	if err != nil {
		return err
	}

	sb.controlLoopConfig = pl.ControlConf
	sb.loop = pl.ControlLoop
	sb.blockNames = pl.BlockNames
	sb.tunedVals = pl.TunedVals

	return nil
}

func (sb *sensorBase) updateControlConfig(
	ctx context.Context, linearValue, angularValue float64,
) error {
	// set linear setpoint config
	if err := control.UpdateConstantBlock(ctx, sb.blockNames[control.BlockNameConstant][0], linearValue, sb.loop); err != nil {
		return err
	}

	// set angular setpoint config
	if err := control.UpdateConstantBlock(ctx, sb.blockNames[control.BlockNameConstant][1], angularValue, sb.loop); err != nil {
		return err
	}

	return nil
}

func sign(x float64) float64 { // A quick helper function
	if math.Signbit(x) {
		return -1.0
	}
	return 1.0
}

// SetState is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// SetVelocity from within that package.
func (sb *sensorBase) SetState(ctx context.Context, state []*control.Signal) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.loop != nil && !sb.loop.Running() {
		return nil
	}

	sb.logger.CDebug(ctx, "setting state")
	linvel := state[0].GetSignalValueAt(0)
	// multiply by the direction of the linear velocity so that angular direction
	// (cw/ccw) doesn't switch when the base is moving backwards
	angvel := (state[1].GetSignalValueAt(0) * sign(linvel))

	return sb.controlledBase.SetPower(ctx, r3.Vector{Y: linvel}, r3.Vector{Z: angvel}, nil)
}

// State is called in endpoint.go of the controls package by the control loop
// instantiated in this file. It is a helper function to call the sensor-controlled base's
// movementsensor and insert its LinearVelocity and AngularVelocity values
// in the signal in the control loop's thread in the endpoint code.
func (sb *sensorBase) State(ctx context.Context) ([]float64, error) {
	sb.logger.CDebug(ctx, "getting state")
	linvel, err := sb.velocities.LinearVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}

	angvel, err := sb.velocities.AngularVelocity(ctx, nil)
	if err != nil {
		return []float64{}, err
	}
	return []float64{linvel.Y, angvel.Z}, nil
}
