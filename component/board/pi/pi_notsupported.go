//go:build !(linux && (arm64 || arm))

package pi

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/board/pi/common"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	registry.RegisterComponent(
		board.Subtype,
		picommon.ModelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return nil, errors.New("not running on a pi")
		}})
	registry.RegisterComponent(
		servo.Subtype,
		picommon.ModelName,
		registry.Component{
			Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Component, logger golog.Logger) (interface{}, error) {
				return nil, errors.New("not running on a pi")
			},
		},
	)
}
