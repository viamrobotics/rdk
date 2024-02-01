package control

import (
	"context"
	"sync"

	"go.uber.org/multierr"
	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/utils"
)

/*
SetupControlLoop
tunePID
GetState ?
SetState ?
*/

// rPiGain is 1/255 because the PWM signal on a pi (and most other boards)
// is limited to 8 bits, or the range 0-255.
const rPiGain = 0.00392157

var (
	// default derivative type is "backward1st1"
	derivativeType = "backward1st1"
	loopFrequency  = 50.0
	sumIndex       = 1
	typeLinVel     = "linear_velocity"
	typeAngVel     = "angular_velocity"
	ogger          = logging.NewLogger("logger")
)

type PIDLoop struct {
	BlockNames              map[string][]string
	ControlConf             Config
	ControlLoop             *Loop
	Options                 Options
	Controllable            Controllable
	PidVals                 []PIDConfig
	logger                  logging.Logger
	activeBackgroundWorkers sync.WaitGroup
}

type PIDConfig struct {
	Type string  `json:"type,omitempty"`
	P    float64 `json:"p"`
	I    float64 `json:"i"`
	D    float64 `json:"d"`
}

// Options contains values used for a control loop.
type Options struct {
	// PositionControlUsingTrapz
	PositionControlUsingTrapz bool

	// SensorFeedbackVelocityControl
	SensorFeedbackVelocityControl bool

	// DerivativeType
	DerivativeType string

	// UseCustomeConfig
	UseCustomConfig bool

	// CompleteCustomConfig
	CompleteCustomConfig Config

	// NeedsAutoTuning
	NeedsAutoTuning bool

	// LoopFrequency
	LoopFrequency float64
}

func SetupPIDControlConfig(pidVals []PIDConfig, componentName string, Options Options, c Controllable, logger logging.Logger) (*PIDLoop, error) {
	pidLoop := &PIDLoop{
		Controllable: c,
		PidVals:      pidVals,
		logger:       logger,
		Options:      Options,
		ControlConf:  Config{},
		ControlLoop:  nil,
	}

	// set controlConf as either an optional custom config, or as the defualt control config
	if Options.UseCustomConfig {
		pidLoop.ControlConf = Options.CompleteCustomConfig
		for i, b := range Options.CompleteCustomConfig.Blocks {
			if b.Type == "sum" {
				sumIndex = i
			}
		}
	} else {
		pidLoop.createControlLoopConfig(pidVals, componentName)
	}

	if Options.NeedsAutoTuning {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		if err := pidLoop.TunePIDLoop(cancelCtx, cancelFunc); err != nil {
			return nil, err
		}
	} else {
		if err := pidLoop.StartControlLoop(); err != nil {
			return nil, err
		}
	}
	// start control loop?
	return pidLoop, nil
}

func (p *PIDLoop) TunePIDLoop(ctx context.Context, cancelFunc context.CancelFunc) error {
	var errs error
	p.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer utils.UncheckedErrorFunc(func() error {
			cancelFunc()
			if p.ControlLoop != nil {
				p.ControlLoop.Stop()
			}
			return nil
		})
		defer p.activeBackgroundWorkers.Done()
		select {
		case <-ctx.Done():
			return
		default:
		}
		// switch sum to depend on the setpoint if position control
		if p.Options.PositionControlUsingTrapz {
			p.ControlConf.Blocks[sumIndex].DependsOn[0] = p.BlockNames["constant"][0]
			if err := p.StartControlLoop(); err != nil {
				errs = multierr.Combine(errs, err)
			}

			if err := p.ControlLoop.MonitorTuning(ctx); err != nil {
				errs = multierr.Combine(errs, err)
			}
		}
		if p.Options.SensorFeedbackVelocityControl {
			// to tune linear PID values, angular PI values must be non-zero
			fakeConf := PIDConfig{Type: typeAngVel, P: 0.5, I: 0.5, D: 0.0}
			p.logger.Info("tuning linear PID")
			if err := p.StartControlLoop(); err != nil {
				errs = multierr.Combine(errs, err)
			}

			if err := p.ControlLoop.MonitorTuning(ctx); err != nil {
				errs = multierr.Combine(errs, err)
			}

			p.ControlLoop.Stop()
			p.ControlLoop = nil

			// to tune angular PID values, linear PI values must be non-zero
			fakeConf.Type = typeLinVel
			p.logger.Info("tuning angular PID")
			if err := p.StartControlLoop(); err != nil {
				errs = multierr.Combine(errs, err)
			}

			if err := p.ControlLoop.MonitorTuning(ctx); err != nil {
				errs = multierr.Combine(errs, err)
			}

			p.ControlLoop.Stop()
			p.ControlLoop = nil
		}

	})
	return errs
}

func (p *PIDLoop) createControlLoopConfig(pidVals []PIDConfig, componentName string) {
	// create basic control config
	p.basicControlConfig(componentName, pidVals[0])

	// add position control
	if p.Options.PositionControlUsingTrapz {
		p.addPositionControl()
	}

	// add sensor feedback velocity control
	if p.Options.SensorFeedbackVelocityControl {
		for i, c := range pidVals {
			if c.Type == "angular_velocity" {
				p.addSensorFeedbackVelocityControl(pidVals[i])
			}
		}
	}

	p.BlockNames = make(map[string][]string, len(p.ControlConf.Blocks))
	// assign block names
	for _, b := range p.ControlConf.Blocks {
		p.BlockNames[string(b.Type)] = append(p.BlockNames[string(b.Type)], b.Name)
	}
}

// create most basic PID control loop containing
// constant -> sum -> PID -> gain -> endpoint -> sum
func (p *PIDLoop) basicControlConfig(endpointName string, pidVals PIDConfig) {
	if p.Options.LoopFrequency != 0 {
		loopFrequency = p.Options.LoopFrequency
	}
	p.ControlConf = Config{
		Blocks: []BlockConfig{
			{
				Name: "set_point",
				Type: "constant",
				Attribute: rdkutils.AttributeMap{
					"constant_val": 0.0,
				},
			},
			{
				Name: "sum",
				Type: "sum",
				Attribute: rdkutils.AttributeMap{
					"sum_string": "+-",
				},
				DependsOn: []string{"set_point", "endpoint"},
			},
			{
				Name: "PID",
				Type: "PID",
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
				Type: "gain",
				Attribute: rdkutils.AttributeMap{
					"gain": rPiGain,
				},
				DependsOn: []string{"PID"},
			},
			{
				Name: "endpoint",
				Type: "endpoint",
				Attribute: rdkutils.AttributeMap{
					"motor_name": endpointName,
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
		Type: "trapezoidalVelocityProfile",
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
	if p.Options.DerivativeType != "" {
		derivativeType = p.Options.DerivativeType
	}
	derivBlock := BlockConfig{
		Name: "derivative",
		Type: "derivative",
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
	for _, b := range p.ControlConf.Blocks {
		if b.Type != blockSum && b.Type != blockEndpoint {
			newName := "linear_" + b.Name
			b.Name = newName
		}
		// change dependsOn to match new name that includes "linear"
		for i, s := range b.DependsOn {
			if s != "sum" && s != "endpoint" {
				newName := "linear_" + s
				b.DependsOn[i] = newName
			}
		}
	}

	// add angular blocks
	// angular constant
	angularSetpoint := BlockConfig{
		Name: "angular_set_point",
		Type: "constant",
		Attribute: rdkutils.AttributeMap{
			"constant_val": 0.0,
		},
		DependsOn: []string{},
	}
	p.ControlConf.Blocks = append(p.ControlConf.Blocks, angularSetpoint)

	// angular PID
	angularPID := BlockConfig{
		Name: "angular_PID",
		Type: "PID",
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

	// angular gain
	angularGain := BlockConfig{
		Name: "angular_gain",
		Type: "gain",
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

func (p *PIDLoop) StartControlLoop() error {
	loop, err := NewLoop(p.logger, p.ControlConf, p.Controllable)
	if err != nil {
		return err
	}
	if err := loop.Start(); err != nil {
		return err
	}
	p.ControlLoop = loop

	return nil
}
