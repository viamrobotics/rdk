// +build pi

// Package pi implements a Board and its related interfaces for a Raspberry Pi.
package pi

// #include <stdlib.h>
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

	"go.viam.com/utils"

	"go.viam.com/core/board"
	"go.viam.com/core/rlog"

	pb "go.viam.com/core/proto/api/v1"
)

var (
	// piHWPinToBroadcom maps the hardware inscribed pin number to
	// its Broadcom pin. For the sake of programming, a user typically
	// knows the hardware pin since they have the board on hand but does
	// not know the corresponding Broadcom pin.
	piHWPinToBroadcom = map[string]uint{
		"3":    2,
		"5":    3,
		"7":    4,
		"11":   17,
		"13":   27,
		"15":   22,
		"19":   10,
		"21":   9,
		"23":   11,
		"29":   5,
		"31":   6,
		"33":   13,
		"35":   19,
		"37":   26,
		"8":    14,
		"10":   15,
		"12":   18,
		"16":   23,
		"18":   24,
		"22":   25,
		"24":   8,
		"26":   7,
		"32":   12,
		"36":   16,
		"38":   20,
		"40":   21,
		"sda":  2,
		"scl":  3,
		"mosi": 10,
		"miso": 9,
		"sclk": 11,
		"ce0":  8,
		"ce1":  7,
		"clk":  18,
	}
)

// init registers a pi board based on pigpio.
func init() {
	board.RegisterBoard("pi", NewPigpio)

	toAdd := map[string]uint{}
	for k, v := range piHWPinToBroadcom {
		if len(k) >= 3 {
			continue
		}
		toAdd[fmt.Sprintf("io%d", v)] = v
	}
	for k, v := range toAdd {
		piHWPinToBroadcom[k] = v
	}
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	cfg           board.Config
	gpioConfigSet map[int]bool
	analogs       map[string]board.AnalogReader
	spis          map[string]board.SPI
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

	initGood := false
	defer func() {
		if !initGood {
			C.gpioTerminate()
			logger.Debug("Pi GPIO terminated due to failed init.")
		}
	}()
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

	// setup SPI buses
	piInstance.spis = map[string]board.SPI{}
	for _, sc := range cfg.SPIs {
		if sc.BusID > 1 {
			return nil, errors.Errorf("Only SPI buses 0 and 1 are available on Pi boards.")
		}
		piInstance.spis[sc.Name] = &piPigpioSPI{pi: piInstance, busID: sc.BusID}
	}

	// setup analogs
	piInstance.analogs = map[string]board.AnalogReader{}
	for _, ac := range cfg.Analogs {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return nil, errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		bus := piInstance.SPI(ac.SPIBus)
		if bus == nil {
			return nil, errors.Errorf("AnalogReader can't find SPI bus named %s", ac.SPIBus)
		}

		ar := &piPigpioAnalogReader{piInstance, channel, bus, ac.ChipSelect}
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
	initGood = true
	return piInstance, nil
}

// GPIOSet sets the given pin to high or low.
func (pi *piPigpio) GPIOSet(pin string, high bool) error {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return errors.Errorf("no hw pin for (%s)", pin)
	}
	return pi.GPIOSetBcom(int(bcom), high)
}

// GPIOGet reads the high/low state of the given pin.
func (pi *piPigpio) GPIOGet(pin string) (bool, error) {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return false, errors.Errorf("no hw pin for (%s)", pin)
	}
	return pi.GPIOGetBcom(int(bcom))
}

// GPIOGetBcom gets the lovel of the given broadcom pin
func (pi *piPigpio) GPIOGetBcom(bcom int) (bool, error) {
	if !pi.gpioConfigSet[bcom] {
		if pi.gpioConfigSet == nil {
			pi.gpioConfigSet = map[int]bool{}
		}
		res := C.gpioSetMode(C.uint(bcom), C.PI_INPUT)
		if res != 0 {
			return false, errors.Errorf("failed to set mode %d", res)
		}
		pi.gpioConfigSet[bcom] = true
	}

	// gpioRead retrns an int 1 or 0, we convert to a bool
	return C.gpioRead(C.uint(bcom)) != 0, nil
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

// PWMSetFreq sets the given pin to the given PWM frequency.
func (pi *piPigpio) PWMSetFreq(pin string, freq uint) error {
	bcom, have := piHWPinToBroadcom[pin]
	if !have {
		return errors.Errorf("no hw pin for (%s)", pin)
	}
	return pi.PWMSetFreqBcom(int(bcom), freq)
}

// PWMSetFreqBcom sets the given broadcom pin to the given PWM frequency.
func (pi *piPigpio) PWMSetFreqBcom(bcom int, freq uint) error {
	if freq == 0 {
		freq = 800 // Original default from libpigpio
	}
	newRes := C.gpioSetPWMfrequency(C.uint(bcom), C.uint(freq))

	if newRes != C.int(freq) {
		return errors.Errorf("pwm set freq fail Tried: %d, got: %d", freq, newRes)
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

// piPigpioAnalogReader implements a board.AnalogReader using an MCP3008 ADC via SPI.
type piPigpioAnalogReader struct {
	pi      *piPigpio
	channel int
	bus     board.SPI
	chip    string
}

func (par *piPigpioAnalogReader) Read(ctx context.Context) (int, error) {
	tx := make([]byte, 3)
	tx[0] = 1                            // start bit
	tx[1] = byte((8 + par.channel) << 4) // single-ended
	tx[2] = 0                            // extra clocks to recieve full 10 bits of data

	par.bus.Lock()
	defer par.bus.Unlock()

	rx, err := par.bus.Xfer(1000000, par.chip, 0, tx)
	if err != nil {
		return 0, err
	}
	val := int(rx[1])<<8 | int(rx[2]) // reassemble 10 bit value

	return val, nil
}

type piPigpioSPI struct {
	pi    *piPigpio
	mu    sync.Mutex
	busID uint
}

func (s *piPigpioSPI) Xfer(baud uint, chipPin string, mode uint, tx []byte) (rx []byte, err error) {

	var spiFlags uint

	if s.busID == 1 {
		spiFlags = spiFlags | 0x100 // Sets AUX SPI bus bit
		if mode == 1 || mode == 3 {
			return nil, errors.New("AUX SPI Bus doesn't support Mode 1 or Mode 3")
		}
	} else if chipPin == "24" || chipPin == "26"{
		return nil, errors.New("Due to underlying issues, use of hardware ChipSelect pins (24 and 26) is not allowed.")
	}

	// Bitfields for mode
	// Mode POL PHA
	// 0    0   0
	// 1    0   1
	// 2    1   0
	// 3    1   1
	spiFlags = spiFlags | mode

	count := len(tx)
	rx = make([]byte, count)
	rxPtr := C.CBytes(rx)
	defer C.free(rxPtr)
	txPtr := C.CBytes(tx)
	defer C.free(txPtr)

	handle := C.spiOpen((C.uint)(0), (C.uint)(baud), (C.uint)(spiFlags))

	if handle < 0 {
		return nil, errors.Errorf("Error opening SPI Bus %d return code was %d, flags were %X", s.busID, handle, spiFlags)
	}
	defer C.spiClose((C.uint)(handle))

	// We're going to directly control chip select (not using CE0/CE1 from SPI controller.)
	// This allows us to use a large number of chips on a single bus.
	// Manually set CS LOW
	err = s.pi.GPIOSet(chipPin, false)
	if err != nil {
		return nil, err
	}
	ret := C.spiXfer((C.uint)(handle), (*C.char)(txPtr), (*C.char)(rxPtr), (C.uint)(count))
	err = s.pi.GPIOSet(chipPin, true)
	if err != nil {
		return nil, err
	}
	if int(ret) != int(count) {
		return nil, errors.Errorf("Error with spiXfer: Wanted %d bytes, got %d bytes.", count, ret)
	}

	rx = C.GoBytes(rxPtr, (C.int)(count))

	return rx, nil
}

func (s *piPigpioSPI) Lock() {
	s.mu.Lock()
	return
}

func (s *piPigpioSPI) Unlock() {
	s.mu.Unlock()
	return
}

// MotorNames returns the name of all known motors.
func (pi *piPigpio) MotorNames() []string {
	names := []string{}
	for k := range pi.motors {
		names = append(names, k)
	}
	return names
}

// SPINames returns the name of all known SPI buses.
func (pi *piPigpio) SPINames() []string {
	names := []string{}
	for k := range pi.spis {
		names = append(names, k)
	}
	return names
}

// ServoNames returns the name of all known servos.
func (pi *piPigpio) ServoNames() []string {
	names := []string{}
	for k := range pi.servos {
		names = append(names, k)
	}
	return names
}

// AnalogReaderNames returns the name of all known analog readers.
func (pi *piPigpio) AnalogReaderNames() []string {
	names := []string{}
	for k := range pi.analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the name of all known digital interrupts.
func (pi *piPigpio) DigitalInterruptNames() []string {
	names := []string{}
	for k := range pi.interrupts {
		names = append(names, k)
	}
	return names
}

func (pi *piPigpio) AnalogReader(name string) board.AnalogReader {
	return pi.analogs[name]
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

func (pi *piPigpio) SPI(name string) board.SPI {
	return pi.spis[name]
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

func (pi *piPigpio) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close() error {

	instanceMu.Lock()
	if len(instances) == 1 {
		C.gpioTerminate()
		pi.logger.Debug("Pi GPIO terminated properly.")
	}
	delete(instances, pi)
	instanceMu.Unlock()

	var err error
	for _, motor := range pi.motors {
		err = multierr.Combine(err, utils.TryClose(motor))
	}

	for _, servo := range pi.servos {
		err = multierr.Combine(err, utils.TryClose(servo))
	}

	for _, spi := range pi.spis {
		err = multierr.Combine(err, utils.TryClose(spi))
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
