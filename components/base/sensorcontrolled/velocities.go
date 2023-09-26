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
		{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: utils.AttributeMap{
				"base_name": "sensor-controlled", // How to input this
			},
			DependsOn: []string{"PID"},
		},
		{
			Name: "pid_block",
			Type: "PID",
			Attribute: utils.AttributeMap{
				"kp": 1.0, // random for now
				"kd": 0.5,
				"kI": 0.2,
			},
			DependsOn: []string{"sumA"},
		},
		{
			Name: "sumA",
			Type: "sum",
			Attribute: utils.AttributeMap{
				"sum_string": "-+", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"sensor-base", "constant"},
		},
		{
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
		{
			Name: "sensor-base",
			Type: "endpoint",
			Attribute: utils.AttributeMap{
				"base_name": "sensor-controlled", // How to input this
			},
			DependsOn: []string{"PID"},
		},
		{
			Name: "pid_block",
			Type: "PID",
			Attribute: utils.AttributeMap{
				"kp": 1.0, // random for now
				"kd": 0.5,
				"kI": 0.2,
			},
			DependsOn: []string{"sumA"},
		},
		{
			Name: "sumA",
			Type: "sum",
			Attribute: utils.AttributeMap{
				"sum_string": "-+", // should this be +- or does it follow dependency order?
			},
			DependsOn: []string{"sensor-base", "constant"},
		},
		{
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
