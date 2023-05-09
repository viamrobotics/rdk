// Package beaglebone implements a beaglebone based board.
package beaglebone

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board/genericlinux"
)

const modelName = "beaglebone"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr genericlinux.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		golog.Global().Debugw("error getting beaglebone GPIO board mapping", "error", err)
	}

	// The false on this line means we're not using Periph. This lets us enable hardware PWM pins.
	genericlinux.RegisterBoard(modelName, gpioMappings, false)
}
