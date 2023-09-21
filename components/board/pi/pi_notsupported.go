//go:build !(linux && (arm64 || arm) && !no_pigpio)

package pi

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/resource"
)

// init registers a failing pi board since this can only be compiled on non-pi systems.
func init() {
	resource.RegisterComponent(
		board.API,
		picommon.Model,
		resource.Registration[board.Board, *genericlinux.Config]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (board.Board, error) {
			return nil, errors.New("not running on a pi")
		}})
	resource.RegisterComponent(
		servo.API,
		picommon.Model,
		resource.Registration[servo.Servo, *picommon.ServoConfig]{
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
