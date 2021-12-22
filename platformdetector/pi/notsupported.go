//go:build !pi
// +build !pi

package pi

import (
	"context"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"

	"go.viam.com/core/component/board"
	"go.viam.com/core/component/servo"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
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
