package control

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
)

// BlockNameEndpoint, BlockNameConstant, and BlockNameTrapezoidal
// represent the strings needed to update a control loop block.
const (
	BlockNameEndpoint    = "endpoint"
	BlockNameConstant    = "constant"
	BlockNameTrapezoidal = "trapezoidalVelocityProfile"
	// rPiGain is 1/255 because the PWM signal on a pi (and most other boards)
	// is limited to 8 bits, or the range 0-255.
	rPiGain                 = 0.00392157
	defaultControllableType = "motor_name"
	defaultDerivativeType   = "backward1st1"
)

var (
	loopFrequency   = 50.0
	sumIndex        = 1
	linearPIDIndex  = 2
	angularPIDIndex = -1
)

// PIDLoop is used for setting up a PID control loop.
type PIDLoop struct {
	BlockNames              map[string][]string
	PIDVals                 []PIDConfig
	TunedVals               *[]PIDConfig
	ControlConf             *Config
	ControlLoop             *Loop
	Options                 Options
	Controllable            Controllable
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup
}

// PIDConfig is values needed to configure a PID control loop.
type PIDConfig struct {
	Type string  `json:"type,omitempty"`
	P    float64 `json:"p"`
	I    float64 `json:"i"`
	D    float64 `json:"d"`
}

// NeedsAutoTuning checks if the PIDConfig values require auto tuning.
func (conf *PIDConfig) NeedsAutoTuning() bool {
	return (conf.P == 0.0 && conf.I == 0.0 && conf.D == 0.0)
}

// Options contains values used for a control loop.
type Options struct {
	// PositionControlUsingTrapz adds a trapezoidalVelocityProfile block to the
	// control config to allow for position control of a component
	PositionControlUsingTrapz bool

	// SensorFeedback2DVelocityControl adds linear and angular blocks to a control
	// config in order to use the sensorcontrolled base component for velocity control
	SensorFeedback2DVelocityControl bool

	// DerivativeType is the type of derivative to be used for the derivative block of a control config
	DerivativeType string

	// UseCustomeConfig is if the necessary config is not created by this setup file
	UseCustomConfig bool

	// CompleteCustomConfig is the custom control config to be used instead of the config
	// created by this setup file
	CompleteCustomConfig Config

	// NeedsAutoTuning is true when all PID values of a PID block are 0 and
	// the control loop needs to be auto-tuned
	NeedsAutoTuning bool

	// LoopFrequency is the frequency at which the control loop should run
	LoopFrequency float64

	// ControllableType is the type of component the control loop will be set up for,
	// currently a base or motor
	ControllableType string
}

// SetupPIDControlConfig creates a control config.
func SetupPIDControlConfig(
	pidVals []PIDConfig,
	componentName string,
	options Options,
	c Controllable,
	logger logging.Logger,
) (*PIDLoop, error) {
	pidLoop := &PIDLoop{
		Controllable: c,
		PIDVals:      pidVals,
		TunedVals:    &[]PIDConfig{{}, {}},
		logger:       logger,
		Options:      options,
		ControlConf:  &Config{},
		ControlLoop:  nil,
	}

	// set controlConf as either an optional custom config, or as the default control config
	if options.UseCustomConfig {
		*pidLoop.ControlConf = options.CompleteCustomConfig
		for i, b := range options.CompleteCustomConfig.Blocks {
			if b.Type == blockSum {
				sumIndex = i
			}
		}
	} else {
		pidLoop.createControlLoopConfig(pidVals, componentName)
	}

	// auto tune the control loop if needed
	if options.NeedsAutoTuning {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		if err := pidLoop.TunePIDLoop(cancelCtx, cancelFunc); err != nil {
			return nil, err
		}
	}

	return pidLoop, nil
}

// TunePIDLoop runs the auto-tuning process for a PID control loop.
func (p *PIDLoop) TunePIDLoop(ctx context.Context, cancelFunc context.CancelFunc) error {
	var errs error
	p.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer func() {
			cancelFunc()
			if p.ControlLoop != nil {
				p.ControlLoop.Stop()
				p.ControlLoop = nil
			}
		}()
		defer p.activeBackgroundWorkers.Done()
		select {
		case <-ctx.Done():
			return
		default:
		}
		if p.Options.UseCustomConfig {
			if err := p.StartControlLoop(); err != nil {
				p.logger.Error(err)
			}
			return
		}
		// switch sum to depend on the setpoint if position control
		if p.Options.PositionControlUsingTrapz {
			p.logger.Debug("tuning trapz PID")
			p.ControlConf.Blocks[sumIndex].DependsOn[0] = p.BlockNames[BlockNameConstant][0]
			if err := p.StartControlLoop(); err != nil {
				errs = multierr.Combine(errs, err)
			}

			p.ControlLoop.MonitorTuning(ctx)

			tunedPID := p.ControlLoop.GetPIDVals(0)
			tunedPID.Type = p.PIDVals[0].Type
			(*p.TunedVals)[0] = tunedPID

			p.ControlLoop.Stop()
			p.ControlLoop = nil
		}
		if p.Options.SensorFeedback2DVelocityControl {
			// check if linear needs to be tuned
			if p.PIDVals[0].NeedsAutoTuning() {
				p.logger.Info("tuning linear PID")
				if err := p.tuneSinglePID(ctx, angularPIDIndex, 0); err != nil {
					errs = multierr.Combine(errs, err)
				}
			}

			// check if angular needs to be tuned
			if p.PIDVals[1].NeedsAutoTuning() {
				p.logger.Info("tuning angular PID")
				if err := p.tuneSinglePID(ctx, linearPIDIndex, 1); err != nil {
					errs = multierr.Combine(errs, err)
				}
			}
		}
	})
	return errs
}

func (p *PIDLoop) tuneSinglePID(ctx context.Context, blockIndex, pidIndex int) error {
	// preserve old values and set them to be non-zero
	pOld := p.ControlConf.Blocks[blockIndex].Attribute["kP"]
	iOld := p.ControlConf.Blocks[blockIndex].Attribute["kI"]
	// to tune one set of PID values, the other PI values must be non-zero
	p.ControlConf.Blocks[blockIndex].Attribute["kP"] = 0.0001
	p.ControlConf.Blocks[blockIndex].Attribute["kI"] = 0.0001
	if err := p.StartControlLoop(); err != nil {
		return err
	}

	p.ControlLoop.MonitorTuning(ctx)
	tunedPID := p.ControlLoop.GetPIDVals(pidIndex)
	tunedPID.Type = p.PIDVals[pidIndex].Type
	(*p.TunedVals)[pidIndex] = tunedPID

	p.ControlLoop.Stop()
	p.ControlLoop = nil

	// reset PI values
	p.ControlConf.Blocks[blockIndex].Attribute["kP"] = pOld
	p.ControlConf.Blocks[blockIndex].Attribute["kI"] = iOld

	return nil
}

func (p *PIDLoop) createControlLoopConfig(pidVals []PIDConfig, componentName string) {
	// create basic control config
	controllableType := defaultControllableType
	if p.Options.ControllableType != "" {
		controllableType = p.Options.ControllableType
	}
	if p.Options.PositionControlUsingTrapz && p.Options.SensorFeedback2DVelocityControl {
		p.logger.Warn(
			"PositionControlUsingTrapz and SensorFeedback2DVelocityControl are not yet supported in the same control loop")
	}

	p.basicControlConfig(componentName, pidVals[0], controllableType)

	// add position control
	if p.Options.PositionControlUsingTrapz {
		p.addPositionControl()
	}

	// add sensor feedback velocity control
	if p.Options.SensorFeedback2DVelocityControl {
		p.addSensorFeedbackVelocityControl(pidVals[1])
	}

	// assign block names
	p.BlockNames = make(map[string][]string, len(p.ControlConf.Blocks))
	for _, b := range p.ControlConf.Blocks {
		p.BlockNames[string(b.Type)] = append(p.BlockNames[string(b.Type)], b.Name)
	}
}

// create most basic PID control loop containing
// constant -> sum -> PID -> gain -> endpoint -> sum.
func (p *PIDLoop) basicControlConfig(endpointName string, pidVals PIDConfig, controllableType string) {
	if p.Options.LoopFrequency != 0.0 {
		loopFrequency = p.Options.LoopFrequency
	}
	*p.ControlConf = Config{
		Blocks: []BlockConfig{
			{
				Name: "set_point",
				Type: blockConstant,
				Attribute: rdkutils.AttributeMap{
					"constant_val": 0.0,
				},
			},
			{
				Name: "sum",
				Type: blockSum,
				Attribute: rdkutils.AttributeMap{
					"sum_string": "+-",
				},
				DependsOn: []string{"set_point", "endpoint"},
			},
			{
				Name: "PID",
				Type: blockPID,
				Attribute: rdkutils.AttributeMap{
					"int_sat_lim_lo": -255.0,
					"int_sat_lim_up": 255.0,
					"kD":             pidVals.D,
					"kI":             pidVals.I,
					"kP":             pidVals.P,
					"limit_lo":       -255.0,
					"limit_up":       255.0,
					"tune_method":    "ziegerNicholsPI",
					"tune_ssr_value": 2.0,
					"tune_step_pct":  0.35,
				},
				DependsOn: []string{"sum"},
			},
			{
				Name: "gain",
				Type: blockGain,
				Attribute: rdkutils.AttributeMap{
					"gain": rPiGain,
				},
				DependsOn: []string{"PID"},
			},
			{
				Name: "endpoint",
				Type: blockEndpoint,
				Attribute: rdkutils.AttributeMap{
					controllableType: endpointName,
				},
				DependsOn: []string{"gain"},
			},
		},
		Frequency: loopFrequency,
	}
}

func (p *PIDLoop) addPositionControl() {
	// add trapezoidalVelocityProfile block between the constant and sum blocks
	trapzBlock := BlockConfig{
		Name: "trapz",
		Type: blockTrapezoidalVelocityProfile,
		Attribute: rdkutils.AttributeMap{
			"kpp_gain":   0.45,
			"max_acc":    30000.0,
			"max_vel":    4000.0,
			"pos_window": 0.0,
		},
		DependsOn: []string{"set_point", "endpoint"},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, trapzBlock)

	// add derivative block between the endpoint and sum blocks
	derivativeType := defaultDerivativeType
	if p.Options.DerivativeType != "" {
		derivativeType = p.Options.DerivativeType
	}
	derivBlock := BlockConfig{
		Name: "derivative",
		Type: blockDerivative,
		Attribute: rdkutils.AttributeMap{
			"derive_type": derivativeType,
		},
		DependsOn: []string{"endpoint"},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, derivBlock)

	p.ControlConf.Blocks[sumIndex].DependsOn[1] = "derivative"
	// change the sum block to depend on the new trapz and derivative blocks
	if !p.Options.NeedsAutoTuning {
		p.ControlConf.Blocks[sumIndex].DependsOn[0] = "trapz"
	}
}

func (p *PIDLoop) addSensorFeedbackVelocityControl(angularPIDVals PIDConfig) {
	// change current block names to include "linear" excluding sum and endpoint
	for i, b := range p.ControlConf.Blocks {
		if b.Type != blockSum && b.Type != blockEndpoint {
			newName := "linear_" + b.Name
			p.ControlConf.Blocks[i].Name = newName
		} else if b.Type == blockSum {
			b.Attribute["sum_string"] = "++-"
		}
		// change dependsOn to match new name that includes "linear"
		for j, s := range b.DependsOn {
			if s != string(blockSum) && s != string(blockEndpoint) {
				newName := "linear_" + s
				b.DependsOn[j] = newName
			}
		}
	}

	// add angular blocks
	// angular constant
	angularSetpoint := BlockConfig{
		Name: "angular_set_point",
		Type: blockConstant,
		Attribute: rdkutils.AttributeMap{
			"constant_val": 0.0,
		},
		DependsOn: []string{},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, angularSetpoint)

	// angular PID
	angularPID := BlockConfig{
		Name: "angular_PID",
		Type: blockPID,
		Attribute: rdkutils.AttributeMap{
			"kD":             angularPIDVals.D,
			"kI":             angularPIDVals.I,
			"kP":             angularPIDVals.P,
			"int_sat_lim_lo": -255.0,
			"int_sat_lim_up": 255.0,
			"limit_lo":       -255.0,
			"limit_up":       255.0,
			"tune_method":    "ziegerNicholsPI",
			"tune_ssr_value": 2.0,
			"tune_step_pct":  0.35,
		},
		DependsOn: []string{"sum"},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, angularPID)
	angularPIDIndex = len(p.ControlConf.Blocks) - 1

	// angular gain
	angularGain := BlockConfig{
		Name: "angular_gain",
		Type: blockGain,
		Attribute: rdkutils.AttributeMap{
			"gain": rPiGain,
		},
		DependsOn: []string{"angular_PID"},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, angularGain)

	// change sum block to depend on the new angular setpoint
	p.ControlConf.Blocks[sumIndex].DependsOn = []string{"linear_set_point", "angular_set_point", "endpoint"}

	// change endpoint block to depend on the new angular gain
	p.ControlConf.Blocks[4].DependsOn = []string{"linear_gain", "angular_gain"}
}

// StartControlLoop starts a PID control loop.
func (p *PIDLoop) StartControlLoop() error {
	loop, err := NewLoop(p.logger, *p.ControlConf, p.Controllable)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	p.ControlLoop = loop

	return nil
}

// CreateConstantBlock returns a new constant block based on the parameters.
func CreateConstantBlock(ctx context.Context, name string, constVal float64) BlockConfig {
	return BlockConfig{
		Name: name,
		Type: blockConstant,
		Attribute: rdkutils.AttributeMap{
			"constant_val": constVal,
		},
		DependsOn: []string{},
	}
}

// UpdateConstantBlock creates and sets a control config constant block.
func UpdateConstantBlock(ctx context.Context, name string, constVal float64, loop *Loop) error {
	newConstBlock := CreateConstantBlock(ctx, name, constVal)
	if err := loop.SetConfigAt(ctx, name, newConstBlock); err != nil {
		return err
	}
	return nil
}

// CreateTrapzBlock returns a new trapezoidalVelocityProfile block based on the parameters.
func CreateTrapzBlock(ctx context.Context, name string, maxVel float64, dependsOn []string) BlockConfig {
	return BlockConfig{
		Name: name,
		Type: blockTrapezoidalVelocityProfile,
		Attribute: rdkutils.AttributeMap{
			"max_vel":    maxVel,
			"max_acc":    30000.0,
			"pos_window": 0.0,
			"kpp_gain":   0.45,
		},
		DependsOn: dependsOn,
	}
}

// UpdateTrapzBlock creates and sets a control config trapezoidalVelocityProfile block.
func UpdateTrapzBlock(ctx context.Context, name string, maxVel float64, dependsOn []string, loop *Loop) error {
	if maxVel == 0 {
		return errors.New("maxVel must be a non-zero value")
	}
	newTrapzBlock := CreateTrapzBlock(ctx, name, maxVel, dependsOn)
	if err := loop.SetConfigAt(ctx, name, newTrapzBlock); err != nil {
		return err
	}
	return nil
}

// TunedPIDErr returns an error with the stored tuned PID values.
func TunedPIDErr(name string, tunedVals []PIDConfig) error {
	var tunedStr string
	for _, pid := range tunedVals {
		if !pid.NeedsAutoTuning() {
			tunedStr += fmt.Sprintf(`{"p": %v, "i": %v, "d": %v, "type": "%v"} `, pid.P, pid.I, pid.D, pid.Type)
		}
	}
	return fmt.Errorf(`%v has been tuned, please copy the following control values into your config: %v`, name, tunedStr)
}

// TuningInProgressErr returns an error when the loop is actively tuning.
func TuningInProgressErr(name string) error {
	return fmt.Errorf(`tuning for %v is in progress`, name)
}
