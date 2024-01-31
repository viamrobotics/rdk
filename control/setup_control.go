package control

import (
	"context"

	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
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
	logger         = logging.NewLogger("logger")
)

type PIDLoop struct {
	Tuning       bool
	ControlConf  Config
	ControlLoop  *Loop
	options      Options
	controllable Controllable
	PidVals      []PIDConfig
	TuningBlocks []*pidTuner
	logger       logging.Logger
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
}

func SetupPIDControlLoop(pidVals []PIDConfig, componentName string, options Options, c Controllable, logger logging.Logger) (*PIDLoop, error) {
	pidLoop := &PIDLoop{
		Tuning:       false,
		controllable: c,
		PidVals:      pidVals,
		logger:       logger,
		options:      options,
	}

	// set controlConf as either an optional custom config, or as the defualt control config
	if options.UseCustomConfig {
		pidLoop.ControlConf = options.CompleteCustomConfig
	} else {
		pidLoop.ControlConf = pidLoop.createControlLoopConfig(pidVals, componentName, options)
	}

	// add auto tuner block
	if options.NeedsAutoTuning {
		pidLoop.TunePIDLoop(context.Background())
	}

	// create control loop
	// loop, err := NewLoop(logger, pidLoop.ControlConf, c)
	// if err != nil {
	// 	return nil, err
	// }
	// pidLoop.ControlLoop = loop

	// start control loop?
	return pidLoop, nil
}

func (p *PIDLoop) TunePIDLoop(ctx context.Context) {
	// switch sum to depend on the setpoint if position control
	if p.options.PositionControlUsingTrapz {
		sumIndex := -1
		for i, b := range p.ControlConf.Blocks {
			if b.Type == "sum" {
				sumIndex = i
			}
		}
		if sumIndex < 0 {
			// ignoring this case for now
			p.logger.Error("no sum block found, one sum block is necessary for auto-tuning")
			return
		}

		// MJ: change these to generic names somehow
		p.ControlConf.Blocks[sumIndex].DependsOn[0] = "set_point"
		p.logger.Errorf("CONTROL CONF AFTER CHANGING SUM = %v", p.ControlConf)
	}

	// create control loop
	loop, err := NewLoop(logger, p.ControlConf, p.controllable)
	if err != nil {
		p.logger.Error(err)
		return
	}
	logger.Error("LOOP CREATED")
	loop.Start()
	logger.Error("LOOP STARTED")
	p.ControlLoop = loop

	// for {
	// 	tuning := p.ControlLoop.GetTuning(ctx)
	// 	if !tuning {
	// 		break
	// 	}
	// }

	// p.logger.Error("done tuning")
}

func (p *PIDLoop) createControlLoopConfig(pidVals []PIDConfig, componentName string, options Options) Config {
	// create basic control config
	p.ControlConf = p.basicControlConfig(componentName, pidVals[0])

	// add position control
	if options.PositionControlUsingTrapz {
		p.ControlConf = p.addPositionControl(p.ControlConf, options.DerivativeType)
	}

	// add sensor feedback velocity control
	if options.SensorFeedbackVelocityControl {
		for i, c := range pidVals {
			if c.Type == "angular_velocity" {
				p.ControlConf = p.addSensorFeedbackVelocityControl(p.ControlConf, pidVals[i])
			}
		}
	}

	return p.ControlConf
}

// create most basic PID control loop containing
// constant -> sum -> PID -> gain -> endpoint -> sum
func (p *PIDLoop) basicControlConfig(endpointName string, pidVals PIDConfig) Config {
	return Config{
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
		Frequency: 50.0,
	}
}

func (p *PIDLoop) addPositionControl(controlConf Config, optionalDerivativeType string) Config {
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
	controlConf.Blocks = append(controlConf.Blocks, trapzBlock)

	// add derivative block between the endpoint and sum blocks
	if optionalDerivativeType != "" {
		derivativeType = optionalDerivativeType
	}
	derivBlock := BlockConfig{
		Name: "derivative",
		Type: "derivative",
		Attribute: rdkutils.AttributeMap{
			"derive_type": derivativeType,
		},
		DependsOn: []string{"endpoint"},
	}
	controlConf.Blocks = append(controlConf.Blocks, derivBlock)

	// change the sum block to depend on the new trapz and derivative blocks
	if !p.options.NeedsAutoTuning {
		controlConf.Blocks[1].DependsOn = []string{"trapz", "derivative"}
	}

	p.logger.Errorf("CONTROL CONF AFTER ADDING POSITION: %v", controlConf)

	return controlConf
}

func (p *PIDLoop) addSensorFeedbackVelocityControl(controlConf Config, angularPIDVals PIDConfig) Config {
	// change current block names to include "linear" excluding sum and endpoint
	for _, b := range controlConf.Blocks {
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
	controlConf.Blocks = append(controlConf.Blocks, angularSetpoint)

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
	controlConf.Blocks = append(controlConf.Blocks, angularPID)

	// angular gain
	angularGain := BlockConfig{
		Name: "angular_gain",
		Type: "gain",
		Attribute: rdkutils.AttributeMap{
			"gain": rPiGain,
		},
		DependsOn: []string{"angular_PID"},
	}
	controlConf.Blocks = append(controlConf.Blocks, angularGain)

	// change sum block to depend on the new angular setpoint
	controlConf.Blocks[1].DependsOn = []string{"linear_set_point", "angular_set_point", "endpoint"}

	// change endpoint block to depend on the new angular gain
	controlConf.Blocks[4].DependsOn = []string{"linear_gain", "angular_gain"}

	return controlConf
}
