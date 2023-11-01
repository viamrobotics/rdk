package gpio

import (
	"context"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

// SetState sets the state of the motor for the built-in control loop.
func (m *EncodedMotor) SetState(ctx context.Context, state []*control.Signal) error {
	power := state[0].GetSignalValueAt(0)
	// scale power input to the 0 to +/- 255 range from the control config
	return m.SetPower(ctx, power/255, nil)
}

// State gets the state of the motor for the built-in control loop.
func (m *EncodedMotor) State(ctx context.Context) ([]float64, error) {
	pos, err := m.position(ctx, nil)
	return []float64{pos}, err
}

// UpdateControlBlockPosVel updates the trap profile and the constant set point for position and velocity control
func (m *EncodedMotor) UpdateControlBlock(ctx context.Context, setPoint float64, maxVel float64) {
	m.logger.Error("IN UPDATE BLOCK")
	m.logger.Errorf("setPoint = %v, maxVel = %v", setPoint, maxVel)
	// Update the Trapezoidal Velocity Profile block with the given maxVel for velocity control
	velConf := control.BlockConfig{
		Name: "trapz",
		Type: control.BlockTrapezoidalVelocityProfile,
		Attribute: rdkutils.AttributeMap{
			"max_vel":    maxVel,
			"max_acc":    30000.0,
			"pos_window": 0.0,
			"kpp_gain":   0.45,
		},
		DependsOn: []string{"set_point", "endpoint"},
	}
	m.loop.SetConfigAt(ctx, "trapz", velConf)

	// Update the Constant block with the given setPoint for position control
	posConf := control.BlockConfig{
		Name: "set_point",
		Type: control.BlockConstant,
		Attribute: rdkutils.AttributeMap{
			"constant_val": setPoint,
		},
		DependsOn: []string{},
	}
	m.loop.SetConfigAt(ctx, "set_point", posConf)
	m.logger.Error("returning")
	return
}

// FindControlBlock starts the control loop and assigns it to m.loop
func (m *EncodedMotor) FindControlBlock() error {
	cLoop, err := control.NewLoop(m.logger, m.cfg.ControlLoop, m)
	if err != nil {
		return err
	}
	err = cLoop.Start()
	if err != nil {
		return err
	}
	m.loop = cLoop
	return nil
}
