// Package beaglebone implements a beaglebone based board.
package beaglebone

import (
	"github.com/edaniels/golog"
	//"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"fmt"

	"go.viam.com/rdk/components/board/genericlinux"
)

const modelName = "beaglebone"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := genericlinux.GetGPIOBoardMappings(modelName, boardInfoMappings)
	fmt.Printf("beaglebone mappings: ->%s<-\n", gpioMappings)
	//var noBoardErr genericlinux.NoBoardFoundError
	if err != nil {
		golog.Global().Debugw("error getting beaglebone GPIO board mapping", "error", err)
	}

	genericlinux.RegisterBoard(modelName, gpioMappings, false)
}
