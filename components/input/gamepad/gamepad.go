//go:build !linux
// +build !linux

// Package gamepad implements a linux gamepad as an input controller.
package gamepad

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/resource"
)

var modelname = resource.DefaultModelFamily.WithModel("gamepad")

func init() {
	resource.RegisterComponent(input.API, modelname, resource.Registration[input.Controller, resource.NoNativeConfig]{
		Constructor: func(ctx context.Context, _ resource.Dependencies, conf resource.Config, logger golog.Logger) (input.Controller, error) {
			return nil, errors.New("gamepad input currently only supported on linux")
		},
	})
}
