//go:build !linux

package genericlinux

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/registry"
)

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
			return nil, errors.New("Linux boards are not supported on non-Linux OSes.")
		}})
}
