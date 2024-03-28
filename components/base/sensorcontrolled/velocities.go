package sensorcontrolled

import (
	"context"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/control"
)

// startControlLoop uses the control config to initialize a control
// loop using the controls package and store in on the sensor controlled base struct
// the sensor base in the controllable interface that implements State and GetState
// called by the endpoint logic of the control thread and the controlLoopConfig
// is included at the end of this file.
func (sb *sensorBase) startControlLoop() error {
	loop, err := control.NewLoop(sb.logger, sb.controlLoopConfig, sb)
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
		LoopFrequency:                   10,
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

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	if len(sb.controlLoopConfig.Blocks) != 0 {
		// if the control loop has not been started or stopped, re-enable it
		if sb.loop == nil {
			if err := sb.startControlLoop(); err != nil {
				return err
			}
		}

		// convert linear.Y mmPerSec to mPerSec, angular.Z is degPerSec
		if err := sb.updateControlConfig(ctx, linear.Y/1000.0, angular.Z); err != nil {
			return err
		}

		return nil
	}

	sb.logger.CInfo(ctx, "setting velocity without loop")
	// else do not use the control loop and pass through the SetVelocity command
	return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
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
