package gpio

import (
	"context"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

// TODO: RSDK-5610 test the scaling factor with a non-pi board with hardware pwm.

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
		Name: m.blockNames["trapz"],
		Type: "trapezoidalVelocityProfile",
		Attribute: rdkutils.AttributeMap{
			"max_vel":    maxVel,
			"max_acc":    30000.0,
			"pos_window": 0.0,
			"kpp_gain":   0.45,
		},
		DependsOn: []string{m.blockNames["constant"], m.blockNames["endpoint"]},
	}
	if err := m.loop.SetConfigAt(ctx, m.blockNames["trapz"], velConf); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	posConf := control.BlockConfig{
		Name: m.blockNames["constant"],
		Type: "constant",
		Attribute: rdkutils.AttributeMap{
			"constant_val": setPoint,
		},
		DependsOn: []string{},
	}
	if err := m.loop.SetConfigAt(ctx, m.blockNames["constant"], posConf); err != nil {
		return err
	}
	return nil
}

// validateControlConfig ensures the programmatically edited blocks are named correctly.
func (m *EncodedMotor) validateControlConfig(ctx context.Context) error {
	m.blockNames = make(map[string]string)
	// verify constant block exists, and store its name
	constBlock, err := m.loop.ConfigAtType(ctx, "constant")
	if err != nil {
		return err
	}
	m.blockNames["constant"] = constBlock[0].Name
	m.logger.Debugf("constant block: %v", constBlock)

	// verify trapezoidalVelocityProfile block exits, and store its name
	trapzBlock, err := m.loop.ConfigAtType(ctx, "trapezoidalVelocityProfile")
	if err != nil {
		return err
	}
	m.blockNames["trapz"] = trapzBlock[0].Name
	m.logger.Debugf("trapz block: %v", trapzBlock)

	// verify sum block exits
	sumBlock, err := m.loop.ConfigAtType(ctx, "sum")
	if err != nil {
		return err
	}
	m.logger.Debugf("sum block: %v", sumBlock)

	// verify PID block exists
	pidBlock, err := m.loop.ConfigAtType(ctx, "PID")
	if err != nil {
		return err
	}
	m.logger.Debugf("PID block: %v", pidBlock)

	// verify endpoint block exists
	endBlock, err := m.loop.ConfigAtType(ctx, "endpoint")
	if err != nil {
		return err
	}
	m.blockNames["endpoint"] = endBlock[0].Name
	m.logger.Debugf("endpoint block: %v", endBlock)

	return nil
}
