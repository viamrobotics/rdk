// Package beaglebone implements a beaglebone based board.
package beaglebone

import (
	"github.com/pkg/errors"
	"periph.io/x/host/v3"

	"go.viam.com/rdk/component/board/commonsysfs"
	"go.viam.com/rdk/rlog"
)

const modelName = "beaglebone"

func init() {
	if _, err := host.Init(); err != nil {
		rlog.Logger.Debugw("error initializing host", "error", err)
	}

	gpioMappings, err := commonsysfs.GetGPIOBoardMappings(modelName, boardInfoMappings)
	var noBoardErr commonsysfs.NoBoardFoundError
	if errors.As(err, &noBoardErr) {
		rlog.Logger.Debugw("error getting beaglebone GPIO board mapping", "error", err)
	}

	rlog.Logger.Debug(
		"beaglebone gpio mapping uses directory sys/devices/platform/bus@100000/*.gpio",
		"\t\t\t\tbeaglebone 600000.gpio/ (128 lines) corresponds to gpiochip1 and /sys/class/gpio/gpiochip300/\n",
		"\t\t\t\tbeaglebone 601000.gpio/ (36 lines) corresponds to gpiochip2 and /sys/class/gpio/gpiochip264/\n",
	)

	rlog.Logger.Debug(
		"beaglebone has 3 pwm chips for the following pins:\n",
		"\t\t\t\t/sys/devices/platform/bus@100000/3000000.pwm/pwm/pwmchip0 for P8_13 and P8_19\n",
		"\t\t\t\t/sys/devices/platform/bus@100000/3020000.pwm/pwm/pwmchip4 for P9_14 and P9_16\n",
		"\t\t\t\t/sys/devices/platform/bus@100000/3010000.pwm/pwm/pwmchip2 for P9_21 and P9_22\n",
	)

	if err == nil {
		commonsysfs.RegisterBoard(modelName, gpioMappings)
	}
}
