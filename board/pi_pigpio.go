// +build pi

package board

// #include <pigpio.h>
// #cgo LDFLAGS: -lpigpio
//
// int doAnalogRead(int h, int channel) {
//     char buf[3];
//     buf[0] = 1;
//     buf[1] = (8+channel) << 8;
//     buf[2] = 0;
//     spiXfer(h, buf, buf, 3);
//     return ((buf[1]&3)<<8) | buf[2];
// }
import "C"

import (
	"fmt"
	"strconv"
)

type piPigpio struct {
	cfg           Config
	analogEnabled bool
	analogSpi     C.int
	gpioConfigSet map[int]bool
	analogs       map[string]AnalogReader
}

func (pi *piPigpio) GetConfig() Config {
	return pi.cfg
}

func (pi *piPigpio) GPIOSet(bcom int, high bool) error {
	if !pi.gpioConfigSet[bcom] {
		if pi.gpioConfigSet == nil {
			pi.gpioConfigSet = map[int]bool{}
		}
		res := C.gpioSetMode(C.uint(bcom), C.PI_OUTPUT)
		if res != 0 {
			return fmt.Errorf("failed to set mode %d", res)
		}
		pi.gpioConfigSet[bcom] = true
	}

	v := 0
	if high {
		v = 1
	}
	C.gpioWrite(C.uint(bcom), C.uint(v))
	return nil
}

type piPigpioAnalogReader struct {
	pi      *piPigpio
	channel int
}

func (par *piPigpioAnalogReader) Read() (int, error) {
	return par.pi.AnalogRead(par.channel)
}

func (pi *piPigpio) AnalogReader(name string) AnalogReader {
	if pi.analogs == nil {
		pi.analogs = map[string]AnalogReader{}
	}

	ar := pi.analogs[name]
	if ar != nil {
		return ar
	}

	for _, ac := range pi.cfg.Analogs {
		if ac.Name != name {
			continue
		}

		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			panic(err)
		}

		ar = &piPigpioAnalogReader{pi, channel}
		ar = AnalogSmootherWrap(ar, ac)

		return ar
	}

	return nil
}

func (pi *piPigpio) AnalogRead(channel int) (int, error) {
	if !pi.analogEnabled {
		pi.analogSpi = C.spiOpen(0, 1000000, 0)
		if pi.analogSpi < 0 {
			return -1, fmt.Errorf("spiOpen failed %d", pi.analogSpi)
		}
		pi.analogEnabled = true
	}

	return int(C.doAnalogRead(pi.analogSpi, C.int(channel))), nil
}

func (pi *piPigpio) Close() error {
	if pi.analogEnabled {
		C.spiClose(C.uint(pi.analogSpi))
		pi.analogSpi = 0
		pi.analogEnabled = false
	}

	C.gpioTerminate()
	piInstance = nil
	return nil
}

var (
	piInstance *piPigpio = nil
)

func NewPigpio(cfg Config) (*piPigpio, error) {
	if piInstance != nil {
		return nil, fmt.Errorf("can only have 1 piPigpio instance")
	}

	internals := C.gpioCfgGetInternals()
	internals |= C.PI_CFG_NOSIGHANDLER
	resCode := C.gpioCfgSetInternals(internals)
	if resCode < 0 {
		return nil, fmt.Errorf("gpioCfgSetInternals failed with code: %d", resCode)
	}

	piInstance = &piPigpio{cfg: cfg}
	resCode = C.gpioInitialise()
	if resCode < 0 {
		return nil, fmt.Errorf("gpioInitialise failed with code: %d", resCode)
	}

	return piInstance, nil
}
