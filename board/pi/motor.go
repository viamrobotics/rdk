//go:build pi
// +build pi

package pi

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/board"
	"go.viam.com/core/config"
	"go.viam.com/core/motor"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
)

// init registers a pi motor based on pigpio.
func init() {
	registry.RegisterMotor(modelName, registry.Motor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
		if err != nil {
			return nil, err
		}

		m, err := board.NewGPIOMotor(actualBoard, *motorConfig, logger)
		if err != nil {
			return nil, err
		}

		m, err = board.WrapMotorWithEncoder(ctx, actualBoard, config, *motorConfig, m, logger)
		if err != nil {
			return nil, err
		}
		return m, nil
	}})
	motor.RegisterConfigAttributeConverter(modelName)

	registry.RegisterMotor("TMC5072", registry.Motor{Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error) {
		actualBoard, motorConfig, err := getBoardFromRobotConfig(r, config)
		if err != nil {
			return nil, err
		}

		m, err := board.NewTMCStepperMotor(ctx, actualBoard, config, *motorConfig, logger)
		if err != nil {
			return nil, err
		}
		return m, nil
	}})
	motor.RegisterConfigAttributeConverter("TMC5072")
}

func getBoardFromRobotConfig(r robot.Robot, config config.Component) (*piPigpio, *motor.Config, error) {
	if !config.Attributes.Has("config") {
		return nil, nil, errors.New("expected config for motor")
	}

	motorConfig := config.Attributes["config"].(*motor.Config)
	if motorConfig.BoardName == "" {
		return nil, nil, errors.New("expected board name in config for motor")
	}
	b, ok := r.BoardByName(motorConfig.BoardName)
	if !ok {
		return nil, nil, fmt.Errorf("expected to find board %q", motorConfig.BoardName)
	}
	// Note(erd): this would not be needed if encoders were a component
	actualBoard, ok := utils.UnwrapProxy(b).(*piPigpio)
	if !ok {
		return nil, nil, errors.New("expected board to be a pi board")
	}
	return actualBoard, motorConfig, nil
}
