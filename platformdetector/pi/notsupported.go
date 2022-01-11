//go:build !(linux && arm64)

// Package pi ensures code for Raspberry Pi platforms can not be used
// on other platforms.
package pi

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const modelName = "pi"

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	registry.RegisterComponent(
		board.Subtype,
		modelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return nil, errors.New("not running on a pi")
		}})
	registry.RegisterComponent(
		servo.Subtype,
		modelName,
		registry.Component{
			Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
				return nil, errors.New("not running on a pi")
			},
		},
	)
}
