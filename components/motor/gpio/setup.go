package gpio

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	vutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("gpio")

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	A             string `json:"a"`
	B             string `json:"b"`
	Direction     string `json:"dir"`
	PWM           string `json:"pwm"`
	EnablePinHigh string `json:"en_high,omitempty"`
	EnablePinLow  string `json:"en_low,omitempty"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             PinConfig      `json:"pins"`
	BoardName        string         `json:"board"`
	MinPowerPct      float64        `json:"min_power_pct,omitempty"` // min power percentage to allow for this motor default is 0.0
	MaxPowerPct      float64        `json:"max_power_pct,omitempty"` // max power percentage to allow for this motor (0.06 - 1.0)
	PWMFreq          uint           `json:"pwm_freq,omitempty"`
	DirectionFlip    bool           `json:"dir_flip,omitempty"`       // Flip the direction of the signal sent if there is a Dir pin
	ControlLoop      control.Config `json:"control_config,omitempty"` // Optional control loop
	Encoder          string         `json:"encoder,omitempty"`        // name of encoder
	RampRate         float64        `json:"ramp_rate,omitempty"`      // how fast to ramp power to motor when using rpm control
	MaxRPM           float64        `json:"max_rpm,omitempty"`
	TicksPerRotation int            `json:"ticks_per_rotation,omitempty"`
	Debug            bool           `json:"rpm_debug,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.BoardName == "" {
		return nil, vutils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, conf.BoardName)

	// If an encoder is present the max_rpm field is optional, in the absence of an encoder the field is required
	if conf.Encoder != "" {
		deps = append(deps, conf.Encoder)
	} else if conf.MaxRPM <= 0 {
		return nil, vutils.NewConfigValidationFieldRequiredError(path, "max_rpm")
	}
	return deps, nil
}

// init registers a pi motor based on pigpio.
func init() {
	comp := registry.Component{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(deps, conf)
			if err != nil {
				return nil, err
			}

			m, err := NewMotor(actualBoard, *motorConfig, conf.ResourceName(), logger)
			if err != nil {
				return nil, err
			}
			if motorConfig.Encoder != "" {
				e, err := encoder.FromDependencies(deps, motorConfig.Encoder)
				if err != nil {
					return nil, err
				}

				m, err = WrapMotorWithEncoder(ctx, e, conf, *motorConfig, m, logger)
				if err != nil {
					return nil, err
				}
			}

			err = m.Stop(ctx, nil)
			if err != nil {
				return nil, err
			}

			return m, nil
		},
	}

	registry.RegisterComponent(motor.Subtype, model, comp)
	config.RegisterComponentAttributeMapConverter(
		motor.Subtype,
		model,
		func(attributes utils.AttributeMap) (interface{}, error) {
			return config.TransformAttributeMapToStruct(&Config{}, attributes)
		},
	)
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
