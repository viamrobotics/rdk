package gpio

import (
	"context"
	"sync"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
)

// SetState sets the state of the motor for the built-in control loop.
func (cm *controlledMotor) SetState(ctx context.Context, state []*control.Signal) error {
	if !cm.loop.Running() {
		return nil
	}
	power := state[0].GetSignalValueAt(0)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.real.SetPower(ctx, power, nil)
}

// State gets the state of the motor for the built-in control loop.
func (cm *controlledMotor) State(ctx context.Context) ([]float64, error) {
	ticks, _, err := cm.enc.Position(ctx, encoder.PositionTypeTicks, nil)
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	pos := ticks + cm.offsetInTicks
	return []float64{pos}, err
}

// updateControlBlockPosVel updates the trap profile and the constant set point for position and velocity control.
func (cm *controlledMotor) updateControlBlock(ctx context.Context, setPoint, maxVel float64) error {
	// Update the Trapezoidal Velocity Profile block with the given maxVel for velocity control
	dependsOn := []string{cm.blockNames[control.BlockNameConstant][0], cm.blockNames[control.BlockNameEndpoint][0]}
	if err := control.UpdateTrapzBlock(ctx, cm.blockNames[control.BlockNameTrapezoidal][0], maxVel, dependsOn, cm.loop); err != nil {
		return err
	}

	// Update the Constant block with the given setPoint for position control
	if err := control.UpdateConstantBlock(ctx, cm.blockNames[control.BlockNameConstant][0], setPoint, cm.loop); err != nil {
		return err
	}

	return nil
}

func (cm *controlledMotor) setupControlLoop() error {
	// set the necessary options for an encoded motor
	options := control.Options{
		PositionControlUsingTrapz: true,
		LoopFrequency:             100.0,
	}

	// convert the motor config ControlParameters to the control.PIDConfig structure for use in setup_control.go
	convertedControlParams := []control.PIDConfig{{
		Type: "",
		P:    cm.cfg.ControlParameters.P,
		I:    cm.cfg.ControlParameters.I,
		D:    cm.cfg.ControlParameters.D,
	}}

	// auto tune motor if all ControlParameters are 0
	// since there's only one set of PID values for a motor, they will always be at convertedControlParams[0]
	if convertedControlParams[0].NeedsAutoTuning() {
		options.NeedsAutoTuning = true
	}

	pl, err := control.SetupPIDControlConfig(convertedControlParams, cm.Name().ShortName(), options, cm, cm.logger)
	if err != nil {
		return err
	}

	cm.controlLoopConfig = pl.ControlConf
	cm.loop = pl.ControlLoop
	cm.blockNames = pl.BlockNames

	return nil
}

func (cm *controlledMotor) startControlLoop() error {
	loop, err := control.NewLoop(cm.logger, cm.controlLoopConfig, cm)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	cm.loop = loop

	return nil
}

func setupMotorWithControls(
	ctx context.Context,
	m motor.Motor,
	enc encoder.Encoder,
	cfg resource.Config,
	logger logging.Logger,
) (motor.Motor, error) {
	conf, err := resource.NativeConfig[*Config](cfg)
	if err != nil {
		return nil, err
	}

	tpr := float64(conf.TicksPerRotation)
	if tpr == 0 {
		tpr = 1.0
	}

	cm := &controlledMotor{
		Named:  cfg.ResourceName().AsNamed(),
		logger: logger,
	}

	return cm, nil
}

type controlledMotor struct {
	resource.Named
	resource.AlwaysRebuild
	logger                  logging.Logger
	opMgr                   *operation.SingleOperationManager
	activeBackGroundWorkers sync.WaitGroup

	offsetInTicks    float64
	ticksPerRotation float64

	mu   sync.RWMutex
	real motor.Motor
	enc  encoder.Encoder

	controlLoopConfig control.Config
	blockNames        map[string][]string
	loop              *control.Loop
	cfg               Config
}
