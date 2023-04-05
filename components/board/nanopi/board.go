// Package nanopi implements a nanopi based board.
// This is an experimental package.
// Supported functionality: GPIO pins, I2C, SPI, Software PWM
// Unsupported functionality: Digital Interrupts, Hardware PWM
package nanopi

import (
	"github.com/edaniels/golog"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/components/board/genericlinux"
)

const modelName = "nanopi"

func init() {
	if _, err := host.Init(); err != nil {
		golog.Global().Debugw("error initializing host", "error", err)
	}
	genericlinux.RegisterBoard(modelName, nil, true)
}
