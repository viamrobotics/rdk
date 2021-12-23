//go:build !linux
// +build !linux

package gamepad

import (
	"context"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

const modelname = "gamepad"

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return nil, errors.New("gamepad input currently only supported on linux")
		}})
}
