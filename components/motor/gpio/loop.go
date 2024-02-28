package gpio

import (
	"context"

	"go.viam.com/rdk/control"
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
	dependsOn := []string{m.blockNames[control.BlockNameConstant][0], m.blockNames[control.BlockNameEndpoint][0]}
	velConf := control.CreateTrapzBlock(ctx, m.blockNames[control.BlockNameTrapezoidal][0], maxVel, dependsOn)
	if err := m.loop.SetConfigAt(ctx, m.blockNames[control.BlockNameTrapezoidal][0], velConf); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	posConf := control.CreateConstantBlock(ctx, m.blockNames[control.BlockNameConstant][0], setPoint)
	if err := m.loop.SetConfigAt(ctx, m.blockNames[control.BlockNameConstant][0], posConf); err != nil {
		return err
	}
	return nil
}

func (m *EncodedMotor) setupControlLoop() error {
	// set the necessary options for an encoded motor
	options := control.Options{
		PositionControlUsingTrapz: true,
		LoopFrequency:             100.0,
	}

	// convert the motor config ControlParameters to the control.PIDConfig structure for use in setup_control.go
	convertedControlParams := []control.PIDConfig{{
		Type: "",
		P:    m.cfg.ControlParameters.P,
		I:    m.cfg.ControlParameters.I,
		D:    m.cfg.ControlParameters.D,
	}}

	// auto tune motor if all ControlParameters are 0
	// since there's only one set of PID values for a motor, they will always be at convertedControlParams[0]
	if convertedControlParams[0].NeedsAutoTuning() {
		options.NeedsAutoTuning = true
	}

	pl, err := control.SetupPIDControlConfig(convertedControlParams, m.Name().ShortName(), options, m, m.logger)
	if err != nil {
		return err
	}

	m.controlLoopConfig = pl.ControlConf
	m.loop = pl.ControlLoop
	m.blockNames = pl.BlockNames

	return nil
}

func (m *EncodedMotor) startControlLoop() error {
	loop, err := control.NewLoop(m.logger, m.controlLoopConfig, m)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	m.loop = loop

	return nil
}
