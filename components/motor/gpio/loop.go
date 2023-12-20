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
