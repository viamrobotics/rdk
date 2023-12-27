package gpio

import (
	"context"
	"errors"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

// TODO: RSDK-5610 test the scaling factor with a non-pi board with hardware pwm.
var (
	errConstantBlock = errors.New("constant block should be called 'set_point")
	errEndpointBlock = errors.New("endpoint block should be called 'endpoint")
	errTrapzBlock    = errors.New("trapezoidalVelocityProfile block should be called 'trapz")
	errPIDBlock      = errors.New("PID block should be called 'PID")
)

// SetState sets the state of the motor for the built-in control loop.
func (m *EncodedMotor) SetState(ctx context.Context, state []*control.Signal) error {
	power := state[0].GetSignalValueAt(0)
	return m.SetPower(ctx, power, nil)
}

// State gets the state of the motor for the built-in control loop.
func (m *EncodedMotor) State(ctx context.Context) ([]float64, error) {
	pos, err := m.position(ctx, nil)
	return []float64{pos}, err
}

// updateControlBlockPosVel updates the trap profile and the constant set point for position and velocity control.
func (m *EncodedMotor) updateControlBlock(ctx context.Context, setPoint, maxVel float64) error {
	// Update the Trapezoidal Velocity Profile block with the given maxVel for velocity control
	velConf := control.BlockConfig{
		Name: "trapz",
		Type: "trapezoidalVelocityProfile",
		Attribute: rdkutils.AttributeMap{
			"max_vel":    maxVel,
			"max_acc":    30000.0,
			"pos_window": 0.0,
			"kpp_gain":   0.45,
		},
		DependsOn: []string{"set_point", "endpoint"},
	}
	if err := m.loop.SetConfigAt(ctx, "trapz", velConf); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	posConf := control.BlockConfig{
		Name: "set_point",
		Type: "constant",
		Attribute: rdkutils.AttributeMap{
			"constant_val": setPoint,
		},
		DependsOn: []string{},
	}
	if err := m.loop.SetConfigAt(ctx, "set_point", posConf); err != nil {
		return err
	}
	return nil
}

// validateControlConfig ensures the programmatically edited blocks are named correctly.
func (m *EncodedMotor) validateControlConfig(ctx context.Context) error {
	constBlock, err := m.loop.ConfigAt(ctx, "set_point")
	if err != nil {
		return errConstantBlock
	}
	m.logger.CDebugf(ctx, "constant block: %v", constBlock)
	endBlock, err := m.loop.ConfigAt(ctx, "endpoint")
	if err != nil {
		return errEndpointBlock
	}
	m.logger.CDebugf(ctx, "endpoint block: %v", endBlock)
	trapzBlock, err := m.loop.ConfigAt(ctx, "trapz")
	if err != nil {
		return errTrapzBlock
	}
	m.logger.CDebugf(ctx, "trapz block: %v", trapzBlock)
	pidBlock, err := m.loop.ConfigAt(ctx, "PID")
	if err != nil {
		return errPIDBlock
	}
	m.logger.CDebugf(ctx, "PID block: %v", pidBlock)
	return nil
}

// Control Loop Configuration is embedded in this file so a user does not have to configure the loop
// from within the attributes of the config file. It sets up a loop that takes a constant ->
// trapezoidalVelocityProfile-> sum -> PID -> gain -> endpoint -> derivative back to sum, and endpoint
// back to trapezoidalVelocityProfile structure. The gain is 0.0039 (1/255) to account for the PID range,
// the PID values are experimental this structure can change as hardware experiments with an encoded motor require.
//
//nolint:unused
var controlLoopConfig = control.Config{
	Blocks: []control.BlockConfig{
		{
			Name: "set_point",
			Type: "constant",
			Attribute: rdkutils.AttributeMap{
				"constant_val": 0,
			},
		},
		{
			Name: "trapz",
			Type: "trapezoidalVelocityProfile",
			Attribute: rdkutils.AttributeMap{
				"kpp_gain":   0.45,
				"max_acc":    30000,
				"max_vel":    4000,
				"pos_window": 0,
			},
			DependsOn: []string{"set_point", "endpoint"},
		},
		{
			Name: "sum",
			Type: "sum",
			Attribute: rdkutils.AttributeMap{
				"sum_string": "+-",
			},
			DependsOn: []string{"trapz", "derivative"},
		},
		{
			Name: "PID",
			Type: "PID",
			Attribute: rdkutils.AttributeMap{
				"int_sat_lim_lo": -255,
				"int_sat_lim_up": 255,
				"kD":             0,
				"kI":             0.53977,
				"kP":             0.048401,
				"limit_lo":       -255,
				"limit_up":       255,
				"tune_method":    "ziegerNicholsSomeOvershoot",
				"tune_ssr_value": 2,
				"tune_step_pct":  0.35,
			},
			DependsOn: []string{"sum"},
		},
		{
			Name: "gain",
			Type: "gain",
			Attribute: rdkutils.AttributeMap{
				"gain": 0.00392156862,
			},
			DependsOn: []string{"PID"},
		},
		{
			Name: "endpoint",
			Type: "endpoint",
			Attribute: rdkutils.AttributeMap{
				"motor_name": "motor",
			},
			DependsOn: []string{"gain"},
		},
		{
			Name: "derivative",
			Type: "derivative",
			Attribute: rdkutils.AttributeMap{
				"derive_type": "backward1st1",
			},
			DependsOn: []string{"endpoint"},
		},
	},
	Frequency: 100,
}
