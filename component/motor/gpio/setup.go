package gpio

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// init registers a pi motor based on pigpio.
func init() {
	comp := registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
			if err != nil {
				return nil, err
			}

			encoderBoard := actualBoard
			if motorConfig.EncoderBoard != "" {
				b, err := board.FromRobot(r, motorConfig.EncoderBoard)
				if err != nil {
					return nil, err
				}
				encoderBoard = b
			}

			m, err := NewMotor(actualBoard, *motorConfig, logger)
			if err != nil {
				return nil, err
			}

			m, err = WrapMotorWithEncoder(ctx, encoderBoard, config, *motorConfig, m, logger)
			if err != nil {
				return nil, err
			}

			err = m.Stop(ctx)
			if err != nil {
				return nil, err
			}

			return m, nil
		},
	}

	registry.RegisterComponent(motor.Subtype, "gpio", comp)
	motor.RegisterConfigAttributeConverter("gpio")

	// for backwards compatibility?
	registry.RegisterComponent(motor.Subtype, "pi", comp)
	motor.RegisterConfigAttributeConverter("pi")
}

func getBoardFromRobotConfig(r robot.Robot, config config.Component) (board.Board, *motor.Config, error) {
	motorConfig, ok := config.ConvertedAttributes.(*motor.Config)
	if !ok {
		return nil, nil, utils.NewUnexpectedTypeError(motorConfig, config.ConvertedAttributes)
	}
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, err := board.FromRobot(r, motorConfig.BoardName)
	if err != nil {
		return nil, nil, err
	}
	return b, motorConfig, nil
}
