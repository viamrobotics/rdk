// +build pi

package pi

// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/edaniels/golog"

	"go.viam.com/robotcore/board"

	pb "go.viam.com/robotcore/proto/api/v1"
)

func init() {
	board.RegisterBoard("pi", NewPigpio)
}

var (
	piHWPinToBroadcom = map[string]uint{
		"3":  2,
		"5":  3,
		"7":  4,
		"11": 17,
		"13": 27,
		"15": 22,
		"19": 10,
		"21": 9,
		"23": 11,
		"29": 5,
		"31": 6,
		"33": 13,
		"35": 19,
		"37": 26,
		"8":  14,
		"10": 15,
		"12": 18,
		"16": 23,
		"18": 24,
		"22": 25,
		"24": 8,
		"26": 7,
		"32": 12,
		"36": 16,
		"38": 20,
		"40": 21,
	}
)

type piPigpio struct {
	cfg           board.Config
	analogEnabled bool
	analogSpi     C.int
	gpioConfigSet map[int]bool
	analogs       map[string]board.AnalogReader
	servos        map[string]board.Servo
	interrupts    map[string]board.DigitalInterrupt
	interruptsHW  map[uint]board.DigitalInterrupt
	motors        map[string]board.Motor
	logger        golog.Logger
}

func (pi *piPigpio) GetConfig(ctx context.Context) (board.Config, error) {
	return pi.cfg, nil
}

func (pi *piPigpio) GPIOSet(pin string, high bool) error {
	//logger.Debugf("GPIOSet %s -> %v", pin, high)
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return fmt.Errorf("no hw pin for (%s)", pin)
	}
	return pi.GPIOSetBcom(int(bcom), high)
}

func (pi *piPigpio) PWMSet(pin string, dutyCycle byte) error {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return fmt.Errorf("no hw pin for (%s)", pin)
	}
	return pi.PWMSetBcom(int(bcom), dutyCycle)
}

func (pi *piPigpio) PWMSetBcom(bcom int, dutyCycle byte) error {
	//logger.Debugf("PWMSetBcom %d -> %d", bcom, dutyCycle)
	res := C.gpioPWM(C.uint(bcom), C.uint(dutyCycle))
	if res != 0 {
		return fmt.Errorf("pwm set fail %d", res)
	}
	return nil
}

func (pi *piPigpio) GPIOSetBcom(bcom int, high bool) error {
	//logger.Debugf("GPIOSetBcom %d -> %v", bcom, high)
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

func (par *piPigpioAnalogReader) Read(ctx context.Context) (int, error) {
	return par.pi.AnalogRead(par.channel)
}

func (pi *piPigpio) AnalogReader(name string) board.AnalogReader {
	return pi.analogs[name]
}

func (pi *piPigpio) AnalogRead(channel int) (int, error) {
	if !pi.analogEnabled {
		pi.analogSpi = C.spiOpen(0, 1000000, 0)
		if pi.analogSpi < 0 {
			return -1, fmt.Errorf("spiOpen failed %d", pi.analogSpi)
		}
		pi.analogEnabled = true
	}

	val := int(C.doAnalogRead(pi.analogSpi, C.int(channel)))
	//logger.Debugf("analog read (%d) %d -> %d", pi.analogSpi, channel, val)
	return val, nil
}

type piPigpioServo struct {
	pi  *piPigpio
	pin C.uint
}

func (s *piPigpioServo) Move(ctx context.Context, angle uint8) error {
	val := 500 + (2000.0 * float64(angle) / 180.0)
	res := C.gpioServo(s.pin, C.uint(val))
	if res != 0 {
		return fmt.Errorf("gpioServo failed with %d", res)
	}
	return nil
}

func (s *piPigpioServo) Current(ctx context.Context) (uint8, error) {
	res := C.gpioGetServoPulsewidth(s.pin)
	if res <= 0 {
		// this includes, errors, we'll ignore
		return 0, nil
	}
	return uint8(180 * (float64(res) - 500.0) / 2000), nil
}

func (pi *piPigpio) Servo(name string) board.Servo {
	return pi.servos[name]
}

func (pi *piPigpio) DigitalInterrupt(name string) board.DigitalInterrupt {
	return pi.interrupts[name]
}

func (pi *piPigpio) Motor(name string) board.Motor {
	return pi.motors[name]
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
func (pi *piPigpio) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return board.CreateStatus(ctx, pi)
}

var (
	piInstance    *piPigpio = nil
	lastTick                = uint32(0)
	tickRollevers           = 0
)

//export pigpioInterruptCallback
func pigpioInterruptCallback(gpio, level int, rawTick uint32) {
	if rawTick < lastTick {
		tickRollevers++
	}
	lastTick = rawTick

	tick := (uint64(tickRollevers) * uint64(math.MaxUint32)) + uint64(rawTick)
	//logger.Debugf("pigpioInterruptCallback gpio: %v level: %v rawTick: %v tick: %v", gpio, level, rawTick, tick)

	i := piInstance.interruptsHW[uint(gpio)]
	if i == nil {
		golog.Global.Infof("no DigitalInterrupt configured for gpio %d", gpio)
		return
	}
	high := true
	if level == 0 {
		high = false
	}
	i.Tick(high, tick*1000)
}

func NewPigpio(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
	var err error
	if piInstance != nil {
		return nil, fmt.Errorf("can only have 1 piPigpio instance")
	}

	// this is so we can run it inside a daemon
	internals := C.gpioCfgGetInternals()
	internals |= C.PI_CFG_NOSIGHANDLER
	resCode := C.gpioCfgSetInternals(internals)
	if resCode < 0 {
		return nil, fmt.Errorf("gpioCfgSetInternals failed with code: %d", resCode)
	}

	// setup
	piInstance = &piPigpio{cfg: cfg}
	resCode = C.gpioInitialise()
	if resCode < 0 {
		return nil, fmt.Errorf("gpioInitialise failed with code: %d", resCode)
	}

	// setup servos
	piInstance.servos = map[string]board.Servo{}
	for _, c := range cfg.Servos {
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			return nil, fmt.Errorf("no hw mapping for %s", c.Pin)
		}

		piInstance.servos[c.Name] = &piPigpioServo{piInstance, C.uint(bcom)}
	}

	// setup analogs
	piInstance.analogs = map[string]board.AnalogReader{}
	for _, ac := range cfg.Analogs {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return nil, fmt.Errorf("bad analog pin (%s)", ac.Pin)
		}

		ar := &piPigpioAnalogReader{piInstance, channel}
		piInstance.analogs[ac.Name] = board.AnalogSmootherWrap(ctx, ar, ac, logger)
	}

	// setup interrupts
	piInstance.interrupts = map[string]board.DigitalInterrupt{}
	piInstance.interruptsHW = map[uint]board.DigitalInterrupt{}
	for _, c := range cfg.DigitalInterrupts {
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			return nil, fmt.Errorf("no hw mapping for %s", c.Pin)
		}

		di, err := board.CreateDigitalInterrupt(c)
		if err != nil {
			return nil, err
		}
		piInstance.interrupts[c.Name] = di
		piInstance.interruptsHW[bcom] = di
		C.setupInterrupt(C.int(bcom))

	}

	// setup motors
	piInstance.motors = map[string]board.Motor{}
	for _, c := range cfg.Motors {
		var m board.Motor
		m, err = board.NewGPIOMotor(piInstance, c.Pins)
		if err != nil {
			return nil, err
		}

		m, err = board.WrapMotorWithEncoder(ctx, piInstance, c, m, logger)
		if err != nil {
			return nil, err
		}

		piInstance.motors[c.Name] = m
	}

	return piInstance, nil
}
