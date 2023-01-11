package uln2003

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

var model = resource.NewDefaultModel("uln")

// PinConfig defines the mapping of where motor are wired.
type PinConfig struct {
	In1       string `json:"In1"`
	In2       string `json:"In2"`
	In3       string `json:"In3"`
	In4       string `json:"In4"`
	Step      string `json:"step"`
	Direction string `json:"dir"`
}

// Config describes the configuration of a motor.
type Config struct {
	Pins             PinConfig `json:"pins"`
	BoardName        string    `json:"board"`
	StepperDelay     uint      `json:"stepper_delay_usec,omitempty"` // When using stepper motors, the time to remain high
	TicksPerRotation int       `json:"ticks_per_rotation"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string
	if config.BoardName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	deps = append(deps, config.BoardName)
	return deps, nil
}

func init() {
	_motor := registry.Component{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(deps, config)
			if err != nil {
				return nil, err
			}

			return newULN(ctx, actualBoard, *motorConfig, config.Name, logger)
		},
	}
	registry.RegisterComponent(motor.Subtype, model, _motor)
	config.RegisterComponentAttributeMapConverter(
		motor.Subtype,
		model,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			return config.TransformAttributeMapToStruct(&conf, attributes)
		},
		&Config{},
	)
}

func getBoardFromRobotConfig(deps registry.Dependencies, config config.Component) (board.Board, *Config, error) {
	motorConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, nil, rdkutils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
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

func newULN(ctx context.Context, b board.Board, mc Config, name string,
	logger golog.Logger,
) (motor.Motor, error) {
	if mc.TicksPerRotation == 0 {
		return nil, errors.New("expected ticks_per_rotation in config for motor")
	}

	m := &ulnStepper{
		theBoard:         b,
		stepsPerRotation: mc.TicksPerRotation,
		stepperDelay:     mc.StepperDelay,
		logger:           logger,
		motorName:        name,
	}

	if mc.Pins.Step != "" {
		stepPin, err := b.GPIOPinByName(mc.Pins.Step)
		if err != nil {
			return nil, err
		}
		m.stepPin = stepPin
	}
	if mc.Pins.Direction != "" {
		directionPin, err := b.GPIOPinByName(mc.Pins.Direction)
		if err != nil {
			return nil, err
		}
		m.dirPin = directionPin
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	m.startThread(ctx)
	return m, nil
}

type ulnStepper struct {
	// config
	theBoard         board.Board
	stepsPerRotation int
	stepperDelay     uint
	stepPin, dirPin  board.GPIOPin
	logger           golog.Logger
	motorName        string

	// state
	lock  sync.Mutex
	opMgr operation.SingleOperationManager

	stepPosition         int64
	threadStarted        bool
	targetStepPosition   int64
	targetStepsPerSecond int64
	generic.Unimplemented
}

func (m *ulnStepper) Validate() error {
	if m.theBoard == nil {
		return errors.New("need a board for gpioStepper")
	}

	if m.stepsPerRotation == 0 {
		return errors.New("need to set 'steps per rotation' for gpioStepper")
	}

	if m.stepperDelay == 0 {
		m.stepperDelay = 20
	}

	if m.stepPin == nil {
		return errors.New("need a 'step' pin for gpioStepper")
	}

	if m.dirPin == nil {
		return errors.New("need a 'dir' pin for gpioStepper")
	}

	return nil
}
