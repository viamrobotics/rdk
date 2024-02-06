// Package odroid implements a odroid based board.
package odroid

import (
	"errors"

	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/logging"
	"periph.io/x/host/v3"
)

const modelName = "odroid"

func init() {
	if _, err := host.Init(); err != nil {
		logging.Global().Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr genericlinux.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		logging.Global().Debugw("error getting odroid GPIO board mapping", "error", err)
	}

	genericlinux.RegisterBoard(modelName, gpioMappings)
}
