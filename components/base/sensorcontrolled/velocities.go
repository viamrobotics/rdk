package sensorcontrolled

import (
	"context"
	"math"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/control"
)

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	ctx, done := sb.opMgr.New(ctx)
	defer done()

	if len(sb.controlLoopConfig.Blocks) == 0 {
		sb.logger.CWarnf(ctx, "control parameters not configured, using %v's SetVelocity method", sb.controlledBase.Name().ShortName())
		return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
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
