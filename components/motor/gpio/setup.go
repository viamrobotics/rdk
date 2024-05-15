package gpio

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/encoder/single"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("gpio")

// MotorType represents the three accepted pin configuration settings
// supported by a gpio motor.
type MotorType int

type pinConfigError int

// ABPwm, DirectionPwm, and AB represent the three pin setups supported by a gpio motor.
const (
	// ABPwm uses A and B direction pins and a pin for pwm signal.
	ABPwm MotorType = iota
	// DirectionPwm uses a single direction pin and a pin for pwm signal.
	DirectionPwm
	// AB uses a pwm signal on pin A for moving forwards and pin B for moving backwards.
	AB

	aNotB pinConfigError = iota
	bNotA
	dirNotPWM
	onlyPWM
	noPins
)

func getPinConfigErrorMessage(errorEnum pinConfigError) error {
	var err error
	switch errorEnum {
	case aNotB:
		err = errors.New("motor pin config has specified pin A but not pin B")
	case bNotA:
		err = errors.New("motor pin config has specified pin B but not pin A")
	case dirNotPWM:
		err = errors.New("motor pin config has direction pin but needs PWM pin")
	case onlyPWM:
		err = errors.New("motor pin config has PWM pin but needs either a direction pin, or A and B pins")
	case noPins:
		err = errors.New("motor pin config devoid of pin definitions (A, B, Direction, PWM are all missing)")
	}
	return err
}

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	A             string `json:"a,omitempty"`
	B             string `json:"b,omitempty"`
	Direction     string `json:"dir,omitempty"`
	PWM           string `json:"pwm,omitempty"`
	EnablePinHigh string `json:"en_high,omitempty"`
	EnablePinLow  string `json:"en_low,omitempty"`
}

// MotorType deduces the type of motor from the pin configuration.
func (conf *PinConfig) MotorType(path string) (MotorType, error) {
	hasA := conf.A != ""
	hasB := conf.B != ""
	hasDir := conf.Direction != ""
	hasPwm := conf.PWM != ""

	var motorType MotorType
	var errEnum pinConfigError

	switch {
	case hasA && hasB:
		if hasPwm {
			motorType = ABPwm
		} else {
			motorType = AB
		}
	case hasDir && hasPwm:
		motorType = DirectionPwm
	case hasA && !hasB:
		errEnum = aNotB
	case hasB && !hasA:
		errEnum = bNotA
	case hasDir && !hasPwm:
		errEnum = dirNotPWM
	case hasPwm && !hasDir && !hasA && !hasB:
		errEnum = onlyPWM
	default:
		errEnum = noPins
	}

	if err := getPinConfigErrorMessage(errEnum); err != nil {
		return motorType, resource.NewConfigValidationError(path, err)
	}
	return motorType, nil
}

type motorPIDConfig struct {
	P float64 `json:"p"`
	I float64 `json:"i"`
	D float64 `json:"d"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins              PinConfig       `json:"pins"`
	BoardName         string          `json:"board"`
	MinPowerPct       float64         `json:"min_power_pct,omitempty"` // min power percentage to allow for this motor default is 0.0
	MaxPowerPct       float64         `json:"max_power_pct,omitempty"` // max power percentage to allow for this motor (0.06 - 1.0)
	PWMFreq           uint            `json:"pwm_freq,omitempty"`
	DirectionFlip     bool            `json:"dir_flip,omitempty"`  // Flip the direction of the signal sent if there is a Dir pin
	Encoder           string          `json:"encoder,omitempty"`   // name of encoder
	RampRate          float64         `json:"ramp_rate,omitempty"` // how fast to ramp power to motor when using rpm control
	MaxRPM            float64         `json:"max_rpm,omitempty"`
	TicksPerRotation  int             `json:"ticks_per_rotation,omitempty"`
	ControlParameters *motorPIDConfig `json:"control_parameters,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.BoardName == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, conf.BoardName)

	// ensure motor config represents one of three supported motor configuration types
	// (see MotorType above)
	if _, err := conf.Pins.MotorType(path); err != nil {
		return deps, err
	}

	// If an encoder is present the max_rpm field is optional, in the absence of an encoder the field is required
	if conf.Encoder != "" {
		if conf.TicksPerRotation <= 0 {
			return nil, resource.NewConfigValidationError(path, errors.New("ticks_per_rotation should be positive or zero"))
		}
		deps = append(deps, conf.Encoder)
	} else if conf.MaxRPM <= 0 {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "max_rpm")
	}
	return deps, nil
}

// init registers a pi motor based on pigpio.
func init() {
	resource.RegisterComponent(motor.API, model, resource.Registration[motor.Motor, *Config]{
		Constructor: createNewMotor,
	})
}

func getBoardFromRobotConfig(deps resource.Dependencies, conf resource.Config) (board.Board, *Config, error) {
	motorConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, nil, err
	}
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, err := board.FromDependencies(deps, motorConfig.BoardName)
	if err != nil {
		return nil, nil, err
	}
	return b, motorConfig, nil
}

func createNewMotor(
	ctx context.Context, deps resource.Dependencies, cfg resource.Config, logger logging.Logger,
) (motor.Motor, error) {
	actualBoard, motorConfig, err := getBoardFromRobotConfig(deps, cfg)
	if err != nil {
		return nil, err
	}

	m, err := NewMotor(actualBoard, *motorConfig, cfg.ResourceName(), logger)
	if err != nil {
		return nil, err
	}

	if motorConfig.Encoder != "" {
		basic := m.(*Motor)
		e, err := encoder.FromDependencies(deps, motorConfig.Encoder)
		if err != nil {
			return nil, err
		}

		props, err := e.Properties(context.Background(), nil)
		if err != nil {
			return nil, errors.New("cannot get encoder properties")
		}
		if !props.TicksCountSupported {
			return nil,
				encoder.NewEncodedMotorPositionTypeUnsupportedError(props)
		}

		single, isSingle := e.(*single.Encoder)
		if isSingle {
			single.AttachDirectionalAwareness(basic)
			logger.CInfo(ctx, "direction attached to single encoder from encoded motor")
		}

		switch {
		case motorConfig.ControlParameters == nil:
			m, err = WrapMotorWithEncoder(ctx, e, cfg, *motorConfig, m, logger)
			if err != nil {
				return nil, err
			}
		default:
			m, err = setupMotorWithControls(ctx, basic, e, cfg, logger)
			if err != nil {
				return nil, err
			}
		}
	}

	err = m.Stop(ctx, nil)
	if err != nil {
		return nil, err
	}

	return m, nil
}
