//go:build !linux
// +build !linux

package gamepad

import (
	"context"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"

	"go.viam.com/core/component/input"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
)

const modelname = "gamepad"

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return nil, errors.New("gamepad input currently only supported on linux")
		}})
}
