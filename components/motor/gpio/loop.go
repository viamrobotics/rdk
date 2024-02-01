package gpio

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/control"
	rdkutils "go.viam.com/rdk/utils"
)

func errMissingBlock(blockType string) error {
	return errors.Errorf("one block of type %s is required", blockType)
}

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
		Name: m.blockNames["trapezoidalVelocityProfile"][0],
		Type: "trapezoidalVelocityProfile",
		Attribute: rdkutils.AttributeMap{
			"max_vel":    maxVel,
			"max_acc":    30000.0,
			"pos_window": 0.0,
			"kpp_gain":   0.45,
		},
		DependsOn: []string{m.blockNames["constant"][0], m.blockNames["endpoint"][0]},
	}
	if err := m.loop.SetConfigAt(ctx, m.blockNames["trapezoidalVelocityProfile"][0], velConf); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	posConf := control.BlockConfig{
		Name: m.blockNames["constant"][0],
		Type: "constant",
		Attribute: rdkutils.AttributeMap{
			"constant_val": setPoint,
		},
		DependsOn: []string{},
	}
	if err := m.loop.SetConfigAt(ctx, m.blockNames["constant"][0], posConf); err != nil {
		return err
	}
	return nil
}

func (m *EncodedMotor) setupControlLoop() error {
	options := control.Options{
		PositionControlUsingTrapz: true,
		LoopFrequency:             100.0,
	}

	if m.cfg.ControlParameters[0].P == 0.0 &&
		m.cfg.ControlParameters[0].I == 0.0 &&
		m.cfg.ControlParameters[0].D == 0.0 {
		options.NeedsAutoTuning = true
	}

	pl, err := control.SetupPIDControlConfig(m.cfg.ControlParameters, m.Name().ShortName(), options, m, m.logger)
	if err != nil {
		return err
	}

	m.controlLoopConfig = pl.ControlConf
	m.loop = pl.ControlLoop
	m.blockNames = pl.BlockNames

	return nil
}
