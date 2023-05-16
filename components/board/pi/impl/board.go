//go:build linux && (arm64 || arm)

// Package piimpl contains the implementation of a supported Raspberry Pi board.
package piimpl

/*
	This driver contains various functionalities of raspberry pi board using the
	pigpio library (https://abyz.me.uk/rpi/pigpio/pdif2.html).
	NOTE: This driver only supports software PWM functionality of raspberry pi.
		  For software PWM, we currently support the default sample rate of
		  5 microseconds, which supports the following 18 frequencies (Hz):
		  8000  4000  2000 1600 1000  800  500  400  320
          250   200   160  100   80   50   40   20   10
		  Details on this can be found here -> https://abyz.me.uk/rpi/pigpio/pdif2.html#set_PWM_frequency
*/

// #include <stdlib.h>
// #include <pigpio.h>
// #include "pi.h"
// #cgo LDFLAGS: -lpigpio
import "C"

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// init registers a pi board based on pigpio.
func init() {
	resource.RegisterComponent(
		board.API,
		picommon.Model,
		resource.Registration[board.Board, *genericlinux.Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (board.Board, error) {
				return newPigpio(ctx, conf.ResourceName(), conf, logger)
			},
		})
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	resource.Named
	resource.AlwaysRebuild
	mu              sync.Mutex
	interruptCtx    context.Context
	interruptCancel context.CancelFunc
	duty            int // added for mutex
	gpioConfigSet   map[int]bool
	analogs         map[string]board.AnalogReader
	i2cs            map[string]board.I2C
	spis            map[string]board.SPI
	// `interrupts` maps interrupt names to the interrupts. `interruptsHW` maps broadcom addresses
	// to these same values.
	interrupts      map[string]board.ReconfigurableDigitalInterrupt
	interruptsHW    map[uint]board.ReconfigurableDigitalInterrupt
	logger          golog.Logger
	isClosed        bool
}

var (
	pigpioInitialized bool
	instanceMu        sync.RWMutex
	instances         = map[*piPigpio]struct{}{}
)

func initializePigpio() error {
	instanceMu.Lock()
	defer instanceMu.Unlock()

	if pigpioInitialized {
		return nil
	}

	resCode := C.gpioInitialise()
	if resCode < 0 {
		// failed to init, check for common causes
		_, err := os.Stat("/sys/bus/platform/drivers/raspberrypi-firmware")
		if err != nil {
			return errors.New("not running on a pi")
		}
		if os.Getuid() != 0 {
			return errors.New("not running as root, try sudo")
		}
		return picommon.ConvertErrorCodeToMessage(int(resCode), "error")
	}

	pigpioInitialized = true
	return nil
}

// newPigpio makes a new pigpio based Board using the given config.
func newPigpio(ctx context.Context, name resource.Name, cfg resource.Config, logger golog.Logger) (board.LocalBoard, error) {
	// this is so we can run it inside a daemon
	internals := C.gpioCfgGetInternals()
	internals |= C.PI_CFG_NOSIGHANDLER
	resCode := C.gpioCfgSetInternals(internals)
	if resCode < 0 {
		return nil, picommon.ConvertErrorCodeToMessage(int(resCode), "gpioCfgSetInternals failed with code")
	}

	if err := initializePigpio(); err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	piInstance := &piPigpio{
		Named:           name.AsNamed(),
		logger:          logger,
		isClosed:        false,
		interruptCtx:    cancelCtx,
		interruptCancel: cancelFunc,
	}

	if err := piInstance.performConfiguration(ctx, nil, cfg); err != nil {
		// This has to happen outside of the lock to avoid a deadlock with interrupts.
		C.gpioTerminate()
		instanceMu.Lock()
		pigpioInitialized = false
		instanceMu.Unlock()
		logger.Error("Pi GPIO terminated due to failed init.")
		return nil, err
	}
	return piInstance, nil
}

// TODO(RSDK-RSDK-2691): implement reconfigure.
func (pi *piPigpio) performConfiguration(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*genericlinux.Config](conf)
	if err != nil {
		return err
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	if err := pi.reconfigureI2cs(ctx, cfg); err != nil {
		return err
	}

	if err := pi.reconfigureSpis(ctx, cfg); err != nil {
		return err
	}

	if err := pi.reconfigureAnalogs(ctx, cfg); err != nil {
		return err
	}

	// This is the only one that actually uses ctx, but we pass it to all previous helpers, too, to
	// keep the interface consistent.
	if err := pi.reconfigureInterrupts(ctx, cfg); err != nil {
		return err
	}

	instanceMu.Lock()
	defer instanceMu.Unlock()
	instances[pi] = struct{}{}
	return nil
}

func (pi *piPigpio) reconfigureI2cs(ctx context.Context, cfg *genericlinux.Config) error {
	pi.i2cs = make(map[string]board.I2C, len(cfg.I2Cs))
	for _, sc := range cfg.I2Cs {
		id, err := strconv.Atoi(sc.Bus)
		if err != nil {
			return err
		}
		pi.i2cs[sc.Name] = &piPigpioI2C{pi: pi, id: id}
	}
	return nil
}

func (pi *piPigpio) reconfigureSpis(ctx context.Context, cfg *genericlinux.Config) error {
	pi.spis = make(map[string]board.SPI, len(cfg.SPIs))
	for _, sc := range cfg.SPIs {
		if sc.BusSelect != "0" && sc.BusSelect != "1" {
			return errors.New("only SPI buses 0 and 1 are available on Pi boards")
		}
		pi.spis[sc.Name] = &piPigpioSPI{pi: pi, busSelect: sc.BusSelect}
	}
	return nil
}

func (pi *piPigpio) reconfigureAnalogs(ctx context.Context, cfg *genericlinux.Config) error {
	pi.analogs = map[string]board.AnalogReader{}
	for _, ac := range cfg.Analogs {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		bus, have := pi.SPIByName(ac.SPIBus)
		if !have {
			return errors.Errorf("can't find SPI bus (%s) requested by AnalogReader", ac.SPIBus)
		}

		ar := &board.MCP3008AnalogReader{channel, bus, ac.ChipSelect}
		pi.analogs[ac.Name] = board.SmoothAnalogReader(ar, ac, pi.logger)
	}
	return nil
}

func findInterruptName(
	interrupt board.ReconfigurableDigitalInterrupt,
	interrupts map[string]board.ReconfigurableDigitalInterrupt,
) (string, bool) {
	for key, value := range interrupts {
		if value == interrupt {
			return key, true
		}
	}

	return "", false
}

func findInterruptBcom(
	interrupt board.ReconfigurableDigitalInterrupt,
	interruptsHW map[uint]board.ReconfigurableDigitalInterrupt,
) (uint, bool) {
	for key, value := range interruptsHW {
		if value == interrupt {
			return key, true
		}
	}

	return 0, false
}

func (pi *piPigpio) reconfigureInterrupts(ctx context.Context, cfg *genericlinux.Config) error {
	// For each old interrupt:
	//     if you're supposed to copy it over, do so
	//     else close it
	// for each new interrupt:
	//     if it exists but is wrong, close it
	//     if it doesn't exist, create it

	// We reuse the old interrupts when possible.
	oldInterrupts := pi.interrupts
	oldInterruptsHW := pi.interruptsHW
	newInterrupts := map[string]board.ReconfigurableDigitalInterrupt{}
	newInterruptsHW := map[uint]board.ReconfigurableDigitalInterrupt{}

	interruptsToClose := make(map[board.ReconfigurableDigitalInterrupt]struct{}, len(oldInterrupts))
	for _, interrupt := range oldInterrupts {
		interruptsToClose[interrupt] = struct{}{}
	} // We'll remove the reused interrupts from this map, and then close the rest.

	for _, newConfig := range cfg.DigitalInterrupts {
		bcom, ok := broadcomPinFromHardwareLabel(newConfig.Pin)
		if !ok {
			return errors.Errorf("no hw mapping for %s", newConfig.Pin)
		}

		// Try reusing an interrupt with the same pin
		if oldInterrupt, ok := oldInterruptsHW[bcom]; ok {
			newInterrupts[newConfig.Name] = oldInterrupt
			newInterruptsHW[bcom] = oldInterrupt
			delete(interruptsToClose, oldInterrupt)
			continue
		}

		// If that didn't work, try reusing an interrupt with the same name
		if oldInterrupt, ok := oldInterrupts[newConfig.Name]; ok {
			newInterrupts[newConfig.Name] = oldInterrupt
			newInterruptsHW[bcom] = oldInterrupt
			delete(interruptsToClose, oldInterrupt)
			continue
		}

		// Otherwise, create the new interrupt from scratch.
		di, err := board.CreateDigitalInterrupt(newConfig)
		if err != nil {
			return err
		}
		newInterrupts[newConfig.Name] = di
		newInterruptsHW[bcom] = di
		if result := C.setupInterrupt(C.int(bcom)); result != 0 {
			return picommon.ConvertErrorCodeToMessage(int(result), "error")
		}
	}

	// For the remaining interrupts, keep any that look implicitly created (interrupts whose name
	// matches its broadcom address), and get rid of the rest.
	for interrupt, _ := range interruptsToClose {
		name, ok := findInterruptName(interrupt, oldInterrupts)
		if !ok {
			// This should never happen
			return errors.Errorf("Logic bug: found old interrupt %s without old name!?", interrupt)
		}

		bcom, ok := findInterruptBcom(interrupt, oldInterruptsHW)
		if !ok {
			// This should never happen, either
			return errors.Errorf("Logic bug: found old interrupt %s without old bcom!?", interrupt)
		}

		if expectedBcom, ok := broadcomPinFromHardwareLabel(name); ok && bcom == expectedBcom {
			// This digital interrupt looks like it was implicitly created. Keep it around!
			newInterrupts[name] = interrupt
			newInterruptsHW[bcom] = interrupt
		} else {
			// This digital interrupt is no longer used.
			interrupt.Close(ctx)
			C.teardownInterrupt(C.int(bcom))
		}
	}

	pi.interrupts = newInterrupts
	pi.interruptsHW = newInterruptsHW
	return nil
}

// GPIOPinNames returns the names of all known GPIO pins.
func (pi *piPigpio) GPIOPinNames() []string {
	names := make([]string, 0, len(piHWPinToBroadcom))
	for k := range piHWPinToBroadcom {
		names = append(names, k)
	}
	return names
}

// GPIOPinByName returns a GPIOPin by name.
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

func (gp gpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	return gp.pi.SetGPIOBcom(gp.bcom, high)
}

func (gp gpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return gp.pi.GetGPIOBcom(gp.bcom)
}

func (gp gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return gp.pi.pwmBcom(gp.bcom)
}

func (gp gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return gp.pi.SetPWMBcom(gp.bcom, dutyCyclePct)
}

func (gp gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return gp.pi.pwmFreqBcom(gp.bcom)
}

func (gp gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
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
			return false, picommon.ConvertErrorCodeToMessage(int(res), "failed to set mode")
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
			return picommon.ConvertErrorCodeToMessage(int(res), "failed to set mode")
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
	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.duty = int(C.gpioPWM(C.uint(bcom), C.uint(dutyCycle)))
	if pi.duty != 0 {
		return errors.Errorf("pwm set fail %d", pi.duty)
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

	if newRes == C.PI_BAD_USER_GPIO {
		return picommon.ConvertErrorCodeToMessage(int(newRes), "pwm set freq failed")
	}

	if newRes != C.int(freqHz) {
		pi.logger.Infof("cannot set pwm freq to %d, setting to closest freq %d", freqHz, newRes)
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
		return nil, errors.New("pi SPI cannot use both native CS pins and extended/gpio CS pins at the same time")
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
		errMsg := fmt.Sprintf("error opening SPI Bus %s, flags were %X", s.bus.busSelect, spiFlags)
		return nil, picommon.ConvertErrorCodeToMessage(int(handle), errMsg)
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
		err = chipPin.Set(ctx, false, nil)
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
		err = chipPin.Set(ctx, true, nil)
		if err != nil {
			return nil, err
		}
	}

	if int(ret) != count {
		return nil, errors.Errorf("error with spiXfer: Wanted %d bytes, got %d bytes", count, ret)
	}

	return C.GoBytes(rxPtr, (C.int)(count)), nil
}

func (s *piPigpioSPI) OpenHandle() (board.SPIHandle, error) {
	s.mu.Lock()
	s.openHandle = &piPigpioSPIHandle{bus: s, isClosed: false}
	return s.openHandle, nil
}

func (s *piPigpioSPI) Close(ctx context.Context) error {
	return nil
}

func (s *piPigpioSPIHandle) Close() error {
	s.isClosed = true
	s.bus.mu.Unlock()
	return nil
}

// SPINames returns the names of all known SPI buses.
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

// I2CNames returns the names of all known SPI buses.
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

// AnalogReaderNames returns the names of all known analog readers.
func (pi *piPigpio) AnalogReaderNames() []string {
	names := []string{}
	for k := range pi.analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (pi *piPigpio) DigitalInterruptNames() []string {
	names := []string{}
	for k := range pi.interrupts {
		names = append(names, k)
	}
	return names
}

// AnalogReaderByName returns an analog reader by name.
func (pi *piPigpio) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := pi.analogs[name]
	return a, ok
}

// SPIByName returns an SPI bus by name.
func (pi *piPigpio) SPIByName(name string) (board.SPI, bool) {
	s, ok := pi.spis[name]
	return s, ok
}

// I2CByName returns an I2C by name.
func (pi *piPigpio) I2CByName(name string) (board.I2C, bool) {
	s, ok := pi.i2cs[name]
	return s, ok
}

// DigitalInterruptByName returns a digital interrupt by name.
// NOTE: During board setup, if a digital interrupt has not been created
// for a pin, then this function will attempt to create one with the pin
// number as the name.
func (pi *piPigpio) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	d, ok := pi.interrupts[name]
	if !ok {
		var err error
		if bcom, have := broadcomPinFromHardwareLabel(name); have {
			if d, ok := pi.interruptsHW[bcom]; ok {
				return d, ok
			}
			d, err = board.CreateDigitalInterrupt(board.DigitalInterruptConfig{
				Name: name,
				Pin:  name,
				Type: "basic",
			})
			if err != nil {
				return nil, false
			}
			pi.interrupts[name] = d
			pi.interruptsHW[bcom] = d
			if result := C.setupInterrupt(C.int(bcom)); result != 0 {
				err := picommon.ConvertErrorCodeToMessage(int(result), "error")
				pi.logger.Errorf("Unable to set up interrupt on pin %s: %s", name, err)
				return nil, false
			}
			return d, true
		}
	}
	return d, ok
}

func (pi *piPigpio) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (pi *piPigpio) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close(ctx context.Context) error {
	var terminate bool
	// Prevent duplicate calls to Close a board as this may overlap with
	// the reinitialization of the board
	pi.mu.Lock()
	if pi.isClosed {
		pi.logger.Info("Duplicate call to close pi board detected, skipping")
		pi.mu.Unlock()
		return nil
	}
	pi.mu.Unlock()
	pi.interruptCancel()
	instanceMu.Lock()
	if len(instances) == 1 {
		terminate = true
	}
	delete(instances, pi)

	if terminate {
		pigpioInitialized = false
		instanceMu.Unlock()
		// This has to happen outside of the lock to avoid a deadlock with interrupts.
		C.gpioTerminate()
		pi.logger.Debug("Pi GPIO terminated properly.")
	} else {
		instanceMu.Unlock()
	}

	var err error
	for _, spi := range pi.spis {
		err = multierr.Combine(err, spi.Close(ctx))
	}
	pi.spis = map[string]board.SPI{}

	for _, analog := range pi.analogs {
		err = multierr.Combine(err, analog.Close(ctx))
	}
	pi.analogs = map[string]board.AnalogReader{}

	for _, interrupt := range pi.interrupts {
		err = multierr.Combine(err, interrupt.Close(ctx))
	}
	pi.interrupts = map[string]board.ReconfigurableDigitalInterrupt{}
	pi.interruptsHW = map[uint]board.ReconfigurableDigitalInterrupt{}

	pi.mu.Lock()
	defer pi.mu.Unlock()
	pi.isClosed = true
	return err
}

// Status returns the current status of the board.
func (pi *piPigpio) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return board.CreateStatus(ctx, pi, extra)
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

	instanceMu.RLock()
	defer instanceMu.RUnlock()
	for instance := range instances {
		i := instance.interruptsHW[uint(gpio)]
		if i == nil {
			golog.Global().Infof("no DigitalInterrupt configured for gpio %d", gpio)
			continue
		}
		high := true
		if level == 0 {
			high = false
		}
		// this should *not* block for long otherwise the lock
		// will be held
		err := i.Tick(instance.interruptCtx, high, tick*1000)
		if err != nil {
			instance.logger.Error(err)
		}
	}
}
