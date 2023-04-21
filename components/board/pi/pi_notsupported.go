//go:build !(linux && (arm64 || arm))

package pi

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/resource"
)

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	resource.RegisterComponent(
		board.API,
		picommon.ModelName,
		resource.Registration[board.Board, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (board.Board, error) {
			return nil, errors.New("not running on a pi")
		}})
	resource.RegisterComponent(
		servo.API,
		picommon.ModelName,
		resource.Registration[servo.Servo, resource.NoNativeConfig]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (servo.Servo, error) {
				return nil, errors.New("not running on a pi")
			},
		},
	)
}
