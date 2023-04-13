// Package intel implements an Intel based board.
package intel

/*
	Datasheet: https://github.com/up-board/up-community/wiki/Pinout_UP4000
	Supported board: UP4000

*/
import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board/genericlinux"
)

const modelName = "intel"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr genericlinux.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		golog.Global().Debugw("error getting up board GPIO board mapping", "error", err)
	}

	genericlinux.RegisterBoard(modelName, gpioMappings, false)
}
