// Package jetson implements a jetson based board.
package jetson

import (
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/component/board/commonsysfs"
	"go.viam.com/rdk/rlog"
)

const modelName = "jetson"

func init() {
	if _, err := host.Init(); err != nil {
		rlog.Logger.Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := commonsysfs.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr commonsysfs.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		rlog.Logger.Debugw("error getting jetson GPIO board mapping", "error", err)
	}

	commonsysfs.RegisterBoard(modelName, gpioMappings)
}
