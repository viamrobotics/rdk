//go:build !linux

// Package genericlinux is for creating board components running Linux. This file, however, is a
// placeholder for when you build the server in a non-Linux environment.
package genericlinux

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// RegisterBoard would register a sysfs based board of the given model. However, this one never
// creates a board, and instead returns errors about making a Linux board on a non-Linux OS.
func RegisterBoard(modelName string, gpioMappings map[string]GPIOBoardMapping) {
	resource.RegisterComponent(
		board.API,
		resource.DefaultModelFamily.WithModel(modelName),
		resource.Registration[board.Board, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (board.Board, error) {
				return nil, errors.New("linux boards are not supported on non-linux OSes")
			},
		})
}

// GetGPIOBoardMappings attempts to find a compatible GPIOBoardMapping for the given board.
func GetGPIOBoardMappings(modelName string, boardInfoMappings map[string]BoardInformation) (map[string]GPIOBoardMapping, error) {
	return nil, errors.New("linux boards are not supported on non-linux OSes")
}
