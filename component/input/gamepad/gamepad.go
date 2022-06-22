//go:build !linux
// +build !linux

// Package gamepad implements a linux gamepad as an input controller.
package gamepad

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

const modelname = "gamepad"

func init() {
	registry.RegisterComponent(input.Subtype, modelname, registry.Component{
		Constructor: func(ctx context.Context, _ registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
			return nil, errors.New("gamepad input currently only supported on linux")
		},
	})
}
