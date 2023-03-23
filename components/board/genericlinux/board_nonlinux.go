//go:build !linux

// Package genericlinux is for creating board components running Linux. This file, however, is a
// placeholder for when you build the server in a non-Linux environment.
package genericlinux

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

// RegisterBoard would register a sysfs based board of the given model. However, this one never
// creates a board, and instead returns errors about making a Linux board on a non-Linux OS.
func RegisterBoard(modelName string, gpioMappings map[int]GPIOBoardMapping, usePeriphGpio bool) {
	registry.RegisterComponent(
		board.Subtype,
		resource.NewDefaultModel(resource.ModelName(modelName)),
		registry.Component{Constructor: func(
			ctx context.Context,
			_ registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return nil, errors.New("linux boards are not supported on non-linux OSes")
		}})
}
