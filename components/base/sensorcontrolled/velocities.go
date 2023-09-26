package sensorcontrolled

import (
	"context"
	"math"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/utils"
)

func (sb *sensorBase) SetVelocity(
	ctx context.Context, linear, angular r3.Vector, extra map[string]interface{},
) error {
	sb.opMgr.CancelRunning(ctx)
	// check if a sensor context has been started
	if sb.sensorLoopDone != nil {
		sb.sensorLoopDone()
	}

	sb.setPolling(true)
	// start a sensor context for the sensor loop based on the longstanding base
	// creator context, and add a timeout for the context
	timeOut := 10 * time.Second
	var sensorCtx context.Context
	sensorCtx, sb.sensorLoopDone = context.WithTimeout(context.Background(), timeOut)

	if err := sb.angularVelocityLoop.Start(); err != nil {
		return err
	}

	if err := sb.linearVelocityLoop.Start(); err != nil {
		return err
	}

	if sb.velocities != nil {
		sb.logger.Warn("not using sensor for SetVelocityfeedback, this feature will be implemented soon")
		// TODO RSDK-3695 implement control loop here instead of placeholder sensor pllling function
		sb.pollsensors(sensorCtx, extra)
		if err := sb.angularVelocityLoop.Start(); err != nil {
			return err
		}
		if err := sb.linearVelocityLoop.Start(); err != nil {
			return err
		}
		return errors.New(
			"setvelocity with sensor feedback not currently implemented, remove movement sensor reporting linear and angular velocity ")
	}
	return sb.controlledBase.SetVelocity(ctx, linear, angular, extra)
}

type linearVelControllable struct {
	control.Controllable // is this necessary?
	sb                   base.Base
	ms                   movementsensor.MovementSensor
}

func (l *linearVelControllable) SetState(ctx context.Context, state float64) error {
	return l.sb.SetVelocity(ctx, r3.Vector{X: 0, Y: state, Z: 0}, r3.Vector{X: 0, Y: 0, Z: 0}, nil)
}

func (l *linearVelControllable) State(ctx context.Context) (float64, error) {
	vel, err := l.ms.LinearVelocity(ctx, nil)
	if err != nil {
		return math.NaN(), err
	}
	return vel.Y, nil
}

type angularVelControllable struct {
	control.Controllable
	sb base.Base
	ms movementsensor.MovementSensor
}

func (l *angularVelControllable) SetState(ctx context.Context, state float64) error {
	return l.sb.SetVelocity(ctx, r3.Vector{X: 0, Y: 0, Z: 0}, r3.Vector{X: 0, Y: 0, Z: state}, nil)
}

func (l *angularVelControllable) State(ctx context.Context) (float64, error) {
	vel, err := l.ms.AngularVelocity(ctx, nil)
	if err != nil {
		return math.NaN(), err
	}
	return vel.Z, nil
}

// Control Loop Cofngiuration

var linearControlAttributes = control.Config{
	Blocks: []control.BlockConfig{
		control.BlockConfig{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: utils.AttributeMap{
				"base_name": "sensor-controlled", // How to input this
			},
			DependsOn: []string{"PID"},
		},
		control.BlockConfig{
			Name: "pid_block",
			Type: "PID",
			Attribute: utils.AttributeMap{
				"kp": 1.0, // random for now
				"kd": 0.5,
				"kI": 0.2,
			},
			DependsOn: []string{"sumA"},
		},
		control.BlockConfig{
			Name: "sumA",
			Type: "sum",
			Attribute: utils.AttributeMap{
				"sum_string": "-+", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"sensor-base", "constant"},
		},
		control.BlockConfig{
			Name: "sumA",
			Type: "constant",
			Attribute: utils.AttributeMap{
				"constant_val": 0.0, // need to update dynamically? Or should I just use the trapezoidal velocit profile
			},
			DependsOn: []string{""},
		},
	},
	Frequency: 50,
}

// these are identical - since my reading of the control code indicates that the endpoint
// sends the SetState and State methods.
var angularControlAttributes = control.Config{
	Blocks: []control.BlockConfig{
		control.BlockConfig{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: utils.AttributeMap{
				"base_name": "sensor-controlled", // How to input this
			},
			DependsOn: []string{"PID"},
		},
		control.BlockConfig{
			Name: "pid_block",
			Type: "PID",
			Attribute: utils.AttributeMap{
				"kp": 1.0, // random for now
				"kd": 0.5,
				"kI": 0.2,
			},
			DependsOn: []string{"sumA"},
		},
		control.BlockConfig{
			Name: "sumA",
			Type: "sum",
			Attribute: utils.AttributeMap{
				"sum_string": "-+", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"sensor-base", "constant"},
		},
		control.BlockConfig{
			Name: "sumA",
			Type: "constant",
			Attribute: utils.AttributeMap{
				"constant_val": 0.0, // need to update dynamically? Or should I just use the trapezoidal velocit profile
			},
			DependsOn: []string{""},
		},
	},
	Frequency: 50,
}
