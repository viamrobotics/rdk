// +build pi

// Package pi implements a Board and its related interfaces for a Raspberry Pi.
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
	"sync"

	"github.com/go-errors/errors"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	"go.viam.com/core/board"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"

	pb "go.viam.com/core/proto/api/v1"
)

// init registers a pi board based on pigpio.
func init() {
	board.RegisterBoard("pi", NewPigpio)
}

var (
	// piHWPinToBroadcom maps the hardware inscribed pin number to
	// its Broadcom pin. For the sake of programming, a user typically
	// knows the hardware pin since they have the board on hand but does
	// not know the corresponding Broadcom pin.
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

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
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

var (
	initOnce   bool
	instanceMu sync.Mutex
	instances  = map[*piPigpio]struct{}{}
)

// NewPigpio makes a new pigpio based Board using the given config.
func NewPigpio(ctx context.Context, cfg board.Config, logger golog.Logger) (board.Board, error) {
	var err error

	// this is so we can run it inside a daemon
	internals := C.gpioCfgGetInternals()
	internals |= C.PI_CFG_NOSIGHANDLER
	resCode := C.gpioCfgSetInternals(internals)
	if resCode < 0 {
		return nil, errors.Errorf("gpioCfgSetInternals failed with code: %d", resCode)
	}

	// setup
	piInstance := &piPigpio{cfg: cfg, logger: logger}

	instanceMu.Lock()
	if !initOnce {
		resCode = C.gpioInitialise()
		if resCode < 0 {
			instanceMu.Unlock()
			return nil, errors.Errorf("gpioInitialise failed with code: %d", resCode)
		}
		initOnce = true
	}
	instanceMu.Unlock()

	// setup servos
	piInstance.servos = map[string]board.Servo{}
	for _, c := range cfg.Servos {
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			return nil, errors.Errorf("no hw mapping for %s", c.Pin)
		}

		piInstance.servos[c.Name] = &piPigpioServo{piInstance, C.uint(bcom)}
	}

	// setup analogs
	piInstance.analogs = map[string]board.AnalogReader{}
	for _, ac := range cfg.Analogs {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return nil, errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		ar := &piPigpioAnalogReader{piInstance, channel}
		piInstance.analogs[ac.Name] = board.SmoothAnalogReader(ar, ac, logger)
	}

	// setup interrupts
	piInstance.interrupts = map[string]board.DigitalInterrupt{}
	piInstance.interruptsHW = map[uint]board.DigitalInterrupt{}
	for _, c := range cfg.DigitalInterrupts {
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			return nil, errors.Errorf("no hw mapping for %s", c.Pin)
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
		m, err = board.NewGPIOMotor(piInstance, c, logger)
		if err != nil {
			return nil, err
		}

		m, err = board.WrapMotorWithEncoder(ctx, piInstance, c, m, logger)
		if err != nil {
			return nil, err
		}

		piInstance.motors[c.Name] = m
	}

	instanceMu.Lock()
	instances[piInstance] = struct{}{}
	instanceMu.Unlock()

	return piInstance, nil
}

// Config returns the config this board was constructed with.
func (pi *piPigpio) Config(ctx context.Context) (board.Config, error) {
	return pi.cfg, nil
}

// GPIOSet sets the given pin to high or low.
func (pi *piPigpio) GPIOSet(pin string, high bool) error {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return errors.Errorf("no hw pin for (%s)", pin)
	}
	return pi.GPIOSetBcom(int(bcom), high)
}

// PWMSet sets the given pin to the given PWM duty cycle.
func (pi *piPigpio) PWMSet(pin string, dutyCycle byte) error {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return errors.Errorf("no hw pin for (%s)", pin)
	}
	return pi.PWMSetBcom(int(bcom), dutyCycle)
}

// PWMSetBcom sets the given broadcom pin to the given PWM duty cycle.
func (pi *piPigpio) PWMSetBcom(bcom int, dutyCycle byte) error {
	res := C.gpioPWM(C.uint(bcom), C.uint(dutyCycle))
	if res != 0 {
		return errors.Errorf("pwm set fail %d", res)
	}
	return nil
}

// GPIOSetBcom sets the given broadcom pin to high or low.
func (pi *piPigpio) GPIOSetBcom(bcom int, high bool) error {
	if !pi.gpioConfigSet[bcom] {
		if pi.gpioConfigSet == nil {
			pi.gpioConfigSet = map[int]bool{}
		}
		res := C.gpioSetMode(C.uint(bcom), C.PI_OUTPUT)
		if res != 0 {
			return errors.Errorf("failed to set mode %d", res)
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

// piPigpioAnalogReader implements a board.AnalogReader using pigpio.
type piPigpioAnalogReader struct {
	pi      *piPigpio
	channel int
}

func (par *piPigpioAnalogReader) Read(ctx context.Context) (int, error) {
	return par.pi.AnalogRead(par.channel)
}

func (par *piPigpioAnalogReader) Reconfigure(newAnalog board.AnalogReader) {
	actual, ok := newAnalog.(*piPigpioAnalogReader)
	if !ok {
		panic(fmt.Errorf("expected new analog to be %T but got %T", actual, newAnalog))
	}
	*par = *actual
}

func (pi *piPigpio) AnalogReader(name string) board.AnalogReader {
	return pi.analogs[name]
}

// AnalogRead read a value on a given channel.
func (pi *piPigpio) AnalogRead(channel int) (int, error) {
	if !pi.analogEnabled {
		pi.analogSpi = C.spiOpen(0, 1000000, 0)
		if pi.analogSpi < 0 {
			return -1, errors.Errorf("spiOpen failed %d", pi.analogSpi)
		}
		pi.analogEnabled = true
	}

	val := int(C.doAnalogRead(pi.analogSpi, C.int(channel)))
	return val, nil
}

// piPigpioServo implements a board.Servo using pigpio.
type piPigpioServo struct {
	pi  *piPigpio
	pin C.uint
}

func (s *piPigpioServo) Move(ctx context.Context, angle uint8) error {
	val := 500 + (2000.0 * float64(angle) / 180.0)
	res := C.gpioServo(s.pin, C.uint(val))
	if res != 0 {
		return errors.Errorf("gpioServo failed with %d", res)
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

func (s *piPigpioServo) Reconfigure(newServo board.Servo) {
	actual, ok := newServo.(*piPigpioServo)
	if !ok {
		panic(fmt.Errorf("expected new servo to be %T but got %T", actual, newServo))
	}
	*s = *actual
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

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close() error {
	if pi.analogEnabled {
		C.spiClose(C.uint(pi.analogSpi))
		pi.analogSpi = 0
		pi.analogEnabled = false
	}

	C.gpioTerminate()
	pi.logger.Debug("Pi GPIO terminated properly.")

	var err error
	for _, motor := range pi.motors {
		err = multierr.Combine(err, utils.TryClose(motor))
	}

	for _, servo := range pi.servos {
		err = multierr.Combine(err, utils.TryClose(servo))
	}

	for _, analog := range pi.analogs {
		err = multierr.Combine(err, utils.TryClose(analog))
	}

	for _, interrupt := range pi.interrupts {
		err = multierr.Combine(err, utils.TryClose(interrupt))
	}

	for _, interruptHW := range pi.interruptsHW {
		err = multierr.Combine(err, utils.TryClose(interruptHW))
	}
	return err
}

func (pi *piPigpio) Status(ctx context.Context) (*pb.BoardStatus, error) {
	return board.CreateStatus(ctx, pi)
}

// assumes we only have one board on host
func (pi *piPigpio) Reconfigure(newBoard board.Board, diff board.ConfigDiff) {
	actual, ok := newBoard.(*piPigpio)
	if !ok {
		panic(fmt.Errorf("expected new base to be %T but got %T", actual, newBoard))
	}

	// removals first here just because of bcom mappings
	for _, c := range diff.Removed.Motors {
		toRemove, ok := pi.motors[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing motor but still reconfiguring", "error", err)
		}
		delete(pi.motors, c.Name)
	}
	for _, c := range diff.Removed.Servos {
		toRemove, ok := pi.servos[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing servo but still reconfiguring", "error", err)
		}
		delete(pi.servos, c.Name)
	}
	for _, c := range diff.Removed.Analogs {
		toRemove, ok := pi.analogs[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing analog but still reconfiguring", "error", err)
		}
		delete(pi.analogs, c.Name)
	}
	for _, c := range diff.Removed.DigitalInterrupts {
		toRemove, ok := pi.interrupts[c.Name]
		if !ok {
			continue // should not happen
		}
		if err := utils.TryClose(toRemove); err != nil {
			rlog.Logger.Errorw("error closing digital interrupt but still reconfiguring", "error", err)
		}
		delete(pi.interrupts, c.Name)
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			panic(errors.Errorf("no hw mapping for %s", c.Pin))
		}
		delete(pi.interruptsHW, bcom)
	}

	if pi.motors == nil && len(diff.Added.Motors) != 0 {
		pi.motors = make(map[string]board.Motor, len(diff.Added.Motors))
	}
	if pi.servos == nil && len(diff.Added.Servos) != 0 {
		pi.servos = make(map[string]board.Servo, len(diff.Added.Servos))
	}
	if pi.analogs == nil && len(diff.Added.Analogs) != 0 {
		pi.analogs = make(map[string]board.AnalogReader, len(diff.Added.Analogs))
	}
	if pi.interrupts == nil && len(diff.Added.DigitalInterrupts) != 0 {
		pi.interrupts = make(map[string]board.DigitalInterrupt, len(diff.Added.DigitalInterrupts))
		pi.interruptsHW = make(map[uint]board.DigitalInterrupt, len(diff.Added.DigitalInterrupts))
	}

	for _, c := range diff.Added.Motors {
		pi.motors[c.Name] = actual.motors[c.Name]
	}
	for _, c := range diff.Added.Servos {
		pi.servos[c.Name] = actual.servos[c.Name]
	}
	for _, c := range diff.Added.Analogs {
		pi.analogs[c.Name] = actual.analogs[c.Name]
	}
	for _, c := range diff.Added.DigitalInterrupts {
		pi.interrupts[c.Name] = actual.interrupts[c.Name]
		bcom, have := piHWPinToBroadcom[c.Pin]
		if !have {
			panic(errors.Errorf("no hw mapping for %s", c.Pin))
		}
		pi.interruptsHW[bcom] = actual.interruptsHW[bcom]
	}

	for _, c := range diff.Modified.Motors {
		pi.motors[c.Name].Reconfigure(actual.motors[c.Name])
	}
	for _, c := range diff.Modified.Servos {
		pi.servos[c.Name].Reconfigure(actual.servos[c.Name])
	}
	for _, c := range diff.Modified.Analogs {
		pi.analogs[c.Name].Reconfigure(actual.analogs[c.Name])
	}
	for _, c := range diff.Modified.DigitalInterrupts {
		pi.interrupts[c.Name].Reconfigure(actual.interrupts[c.Name])
	}

	pi.cfg = actual.cfg
	pi.analogEnabled = actual.analogEnabled
	pi.analogSpi = actual.analogSpi
	pi.gpioConfigSet = actual.gpioConfigSet
	pi.logger = actual.logger
}

var (
	lastTick      = uint32(0)
	tickRollevers = 0
)

//export pigpioInterruptCallback
func pigpioInterruptCallback(gpio, level int, rawTick uint32) {
	if rawTick < lastTick {
		tickRollevers++
	}
	lastTick = rawTick

	tick := (uint64(tickRollevers) * uint64(math.MaxUint32)) + uint64(rawTick)

	instanceMu.Lock()
	defer instanceMu.Unlock()
	for instance := range instances {
		i := instance.interruptsHW[uint(gpio)]
		if i == nil {
			rlog.Logger.Infof("no DigitalInterrupt configured for gpio %d", gpio)
			continue
		}
		high := true
		if level == 0 {
			high = false
		}
		// this should *not* block for long otherwise the lock
		// will be held
		i.Tick(high, tick*1000)
	}
}
