//go:build linux && (arm64 || arm)

// Package piimpl contains the implementation of a supported Raspberry Pi board.
package piimpl

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"math"
	"os"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	picommon "go.viam.com/rdk/component/board/pi/common"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	rdkutils "go.viam.com/rdk/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// init registers a pi board based on pigpio.
func init() {
	registry.RegisterComponent(
		board.Subtype,
		picommon.ModelName,
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			boardConfig, ok := config.ConvertedAttributes.(*board.Config)
			if !ok {
				return nil, rdkutils.NewUnexpectedTypeError(boardConfig, config.ConvertedAttributes)
			}
			return NewPigpio(ctx, boardConfig, logger)
		}})
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	generic.Unimplemented
	mu            sync.Mutex
	cfg           *board.Config
	gpioConfigSet map[int]bool
	analogs       map[string]board.AnalogReader
	i2cs          map[string]board.I2C
	spis          map[string]board.SPI
	interrupts    map[string]board.DigitalInterrupt
	interruptsHW  map[uint]board.DigitalInterrupt
	logger        golog.Logger
}

var (
	initOnce   bool
	instanceMu sync.Mutex
	instances  = map[*piPigpio]struct{}{}
)

// NewPigpio makes a new pigpio based Board using the given config.
func NewPigpio(ctx context.Context, cfg *board.Config, logger golog.Logger) (board.LocalBoard, error) {
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
			// failed to init, check for common causes
			_, err := os.Stat("/sys/bus/platform/drivers/raspberrypi-firmware")
			if err != nil {
				return nil, errors.New("not running on a pi")
			}
			if os.Getuid() != 0 {
				return nil, errors.New("not running as root, try sudo")
			}
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

	// setup I2C buses
	if len(cfg.I2Cs) != 0 {
		piInstance.i2cs = make(map[string]board.I2C, len(cfg.I2Cs))
		for _, sc := range cfg.I2Cs {
			id, err := strconv.Atoi(sc.Bus)
			if err != nil {
				return nil, err
			}
			piInstance.i2cs[sc.Name] = &piPigpioI2C{pi: piInstance, id: id}
		}
	}

	// setup SPI buses
	if len(cfg.SPIs) != 0 {
		piInstance.spis = make(map[string]board.SPI, len(cfg.SPIs))
		for _, sc := range cfg.SPIs {
			if sc.BusSelect != "0" && sc.BusSelect != "1" {
				return nil, errors.New("only SPI buses 0 and 1 are available on Pi boards.")
			}
			piInstance.spis[sc.Name] = &piPigpioSPI{pi: piInstance, busSelect: sc.BusSelect}
		}
	}

	// setup analogs
	piInstance.analogs = map[string]board.AnalogReader{}
	for _, ac := range cfg.Analogs {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return nil, errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		bus, have := piInstance.SPIByName(ac.SPIBus)
		if !have {
			return nil, errors.Errorf("can't find SPI bus (%s) requested by AnalogReader", ac.SPIBus)
		}

		ar := &board.MCP3008AnalogReader{channel, bus, ac.ChipSelect}
		piInstance.analogs[ac.Name] = board.SmoothAnalogReader(ar, ac, logger)
	}

	// setup interrupts
	piInstance.interrupts = map[string]board.DigitalInterrupt{}
	piInstance.interruptsHW = map[uint]board.DigitalInterrupt{}
	for _, c := range cfg.DigitalInterrupts {
		bcom, have := broadcomPinFromHardwareLabel(c.Pin)
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

	instanceMu.Lock()
	instances[piInstance] = struct{}{}
	instanceMu.Unlock()
	initGood = true
	return piInstance, nil
}

func (pi *piPigpio) GPIOPinNames() []string {
	names := make([]string, 0, len(piHWPinToBroadcom))
	for k := range piHWPinToBroadcom {
		names = append(names, k)
	}
	return names
}

func (pi *piPigpio) GPIOPinByName(pin string) (board.GPIOPin, error) {
	bcom, have := broadcomPinFromHardwareLabel(pin)
	if !have {
		return nil, errors.Errorf("no hw pin for (%s)", pin)
	}
	return gpioPin{pi, int(bcom)}, nil
}

type gpioPin struct {
	pi   *piPigpio
	bcom int
}

func (gp gpioPin) Set(ctx context.Context, high bool) error {
	return gp.pi.SetGPIOBcom(gp.bcom, high)
}

func (gp gpioPin) Get(ctx context.Context) (bool, error) {
	return gp.pi.GetGPIOBcom(gp.bcom)
}

func (gp gpioPin) PWM(ctx context.Context) (float64, error) {
	return gp.pi.pwmBcom(gp.bcom)
}

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64) error {
	return gp.pi.SetPWMBcom(gp.bcom, dutyCyclePct)
}

func (gp gpioPin) PWMFreq(ctx context.Context) (uint, error) {
	return gp.pi.pwmFreqBcom(gp.bcom)
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint) error {
	return gp.pi.SetPWMFreqBcom(gp.bcom, freqHz)
}

// GetGPIOBcom gets the level of the given broadcom pin
func (pi *piPigpio) GetGPIOBcom(bcom int) (bool, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
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

// SetGPIOBcom sets the given broadcom pin to high or low.
func (pi *piPigpio) SetGPIOBcom(bcom int, high bool) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()
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

func (pi *piPigpio) pwmBcom(bcom int) (float64, error) {
	res := C.gpioGetPWMdutycycle(C.uint(bcom))
	return float64(res) / 255, nil
}

// SetPWMBcom sets the given broadcom pin to the given PWM duty cycle.
func (pi *piPigpio) SetPWMBcom(bcom int, dutyCyclePct float64) error {
	dutyCycle := rdkutils.ScaleByPct(255, dutyCyclePct)
	res := C.gpioPWM(C.uint(bcom), C.uint(dutyCycle))
	if res != 0 {
		return errors.Errorf("pwm set fail %d", res)
	}
	return nil
}

func (pi *piPigpio) pwmFreqBcom(bcom int) (uint, error) {
	res := C.gpioGetPWMfrequency(C.uint(bcom))
	return uint(res), nil
}

// SetPWMFreqBcom sets the given broadcom pin to the given PWM frequency.
func (pi *piPigpio) SetPWMFreqBcom(bcom int, freqHz uint) error {
	if freqHz == 0 {
		freqHz = 800 // Original default from libpigpio
	}
	newRes := C.gpioSetPWMfrequency(C.uint(bcom), C.uint(freqHz))

	if newRes != C.int(freqHz) {
		return errors.Errorf("pwm set freq fail Tried: %d, got: %d", freqHz, newRes)
	}
	return nil
}

type piPigpioSPI struct {
	pi           *piPigpio
	mu           sync.Mutex
	busSelect    string
	openHandle   *piPigpioSPIHandle
	nativeCSSeen bool
	gpioCSSeen   bool
}

type piPigpioSPIHandle struct {
	bus      *piPigpioSPI
	isClosed bool
}

func (s *piPigpioSPIHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) ([]byte, error) {
	if s.isClosed {
		return nil, errors.New("can't use Xfer() on an already closed SPIHandle")
	}

	var spiFlags uint
	var gpioCS bool
	var nativeCS C.uint

	if s.bus.busSelect == "1" {
		spiFlags |= 0x100 // Sets AUX SPI bus bit
		if mode == 1 || mode == 3 {
			return nil, errors.New("AUX SPI Bus doesn't support Mode 1 or Mode 3")
		}
		if chipSelect == "11" || chipSelect == "12" || chipSelect == "36" {
			s.bus.nativeCSSeen = true
			if chipSelect == "11" {
				nativeCS = 1
			} else if chipSelect == "36" {
				nativeCS = 2
			}
		} else {
			s.bus.gpioCSSeen = true
			gpioCS = true
		}
	} else {
		if chipSelect == "24" || chipSelect == "26" {
			s.bus.nativeCSSeen = true
			if chipSelect == "26" {
				nativeCS = 1
			}
		} else {
			s.bus.gpioCSSeen = true
			gpioCS = true
		}
	}

	// Libpigpio will always enable the native CS output on 24 & 26 (or 11, 12, & 36 for aux SPI)
	// Thus you don't have anything using those pins even when we're directly controlling another (extended/gpio) CS line
	// Use only the native CS pins OR don't use them at all
	if s.bus.nativeCSSeen && s.bus.gpioCSSeen {
		return nil, errors.New("Pi SPI cannot use both native CS pins and extended/gpio CS pins at the same time.")
	}

	// Bitfields for mode
	// Mode POL PHA
	// 0    0   0
	// 1    0   1
	// 2    1   0
	// 3    1   1
	spiFlags |= mode

	count := len(tx)
	rx := make([]byte, count)
	rxPtr := C.CBytes(rx)
	defer C.free(rxPtr)
	txPtr := C.CBytes(tx)
	defer C.free(txPtr)

	handle := C.spiOpen(nativeCS, (C.uint)(baud), (C.uint)(spiFlags))

	if handle < 0 {
		return nil, errors.Errorf("error opening SPI Bus %s return code was %d, flags were %X", s.bus.busSelect, handle, spiFlags)
	}
	defer C.spiClose((C.uint)(handle))

	if gpioCS {
		// We're going to directly control chip select (not using CE0/CE1/CE2 from SPI controller.)
		// This allows us to use a large number of chips on a single bus.
		// Per "seen" checks above, cannot be mixed with the native CE0/CE1/CE2
		chipPin, err := s.bus.pi.GPIOPinByName(chipSelect)
		if err != nil {
			return nil, err
		}
		err = chipPin.Set(ctx, false)
		if err != nil {
			return nil, err
		}
	}

	ret := C.spiXfer((C.uint)(handle), (*C.char)(txPtr), (*C.char)(rxPtr), (C.uint)(count))

	if gpioCS {
		chipPin, err := s.bus.pi.GPIOPinByName(chipSelect)
		if err != nil {
			return nil, err
		}
		err = chipPin.Set(ctx, true)
		if err != nil {
			return nil, err
		}
	}

	if int(ret) != count {
		return nil, errors.Errorf("error with spiXfer: Wanted %d bytes, got %d bytes.", count, ret)
	}

	return C.GoBytes(rxPtr, (C.int)(count)), nil
}

func (s *piPigpioSPI) OpenHandle() (board.SPIHandle, error) {
	s.mu.Lock()
	s.openHandle = &piPigpioSPIHandle{bus: s, isClosed: false}
	return s.openHandle, nil
}

func (h *piPigpioSPIHandle) Close() error {
	h.isClosed = true
	h.bus.mu.Unlock()
	return nil
}

// SPINames returns the name of all known SPI buses.
func (pi *piPigpio) SPINames() []string {
	if len(pi.spis) == 0 {
		return nil
	}
	names := make([]string, 0, len(pi.spis))
	for k := range pi.spis {
		names = append(names, k)
	}
	return names
}

// I2CNames returns the name of all known SPI buses.
func (pi *piPigpio) I2CNames() []string {
	if len(pi.i2cs) == 0 {
		return nil
	}
	names := make([]string, 0, len(pi.i2cs))
	for k := range pi.i2cs {
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

func (pi *piPigpio) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := pi.analogs[name]
	return a, ok
}

func (pi *piPigpio) SPIByName(name string) (board.SPI, bool) {
	s, ok := pi.spis[name]
	return s, ok
}

func (pi *piPigpio) I2CByName(name string) (board.I2C, bool) {
	s, ok := pi.i2cs[name]
	return s, ok
}

func (pi *piPigpio) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	d, ok := pi.interrupts[name]
	return d, ok
}

func (pi *piPigpio) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close(ctx context.Context) error {
	instanceMu.Lock()
	if len(instances) == 1 {
		C.gpioTerminate()
		pi.logger.Debug("Pi GPIO terminated properly.")
	}
	delete(instances, pi)
	instanceMu.Unlock()

	var err error
	for _, spi := range pi.spis {
		err = multierr.Combine(err, utils.TryClose(ctx, spi))
	}

	for _, analog := range pi.analogs {
		err = multierr.Combine(err, utils.TryClose(ctx, analog))
	}

	for _, interrupt := range pi.interrupts {
		err = multierr.Combine(err, utils.TryClose(ctx, interrupt))
	}

	for _, interruptHW := range pi.interruptsHW {
		err = multierr.Combine(err, utils.TryClose(ctx, interruptHW))
	}
	return err
}

func (pi *piPigpio) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
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
		// TODO(RDK-37): use new cgo Value to pass a context?
		err := i.Tick(context.TODO(), high, tick*1000)
		if err != nil {
			instance.logger.Error(err)
		}
	}
}
