//go:build linux && (arm64 || arm) && !no_pigpio && !no_cgo

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

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/board/mcp3008helper"
	picommon "go.viam.com/rdk/components/board/pi/common"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// init registers a pi board based on pigpio.
func init() {
	resource.RegisterComponent(
		board.API,
		picommon.Model,
		resource.Registration[board.Board, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger logging.Logger,
			) (board.Board, error) {
				return newPigpio(ctx, conf.ResourceName(), conf, logger)
			},
		})
}

// A Config describes the configuration of a board and all of its connected parts.
type Config struct {
	AnalogReaders     []mcp3008helper.MCP3008AnalogConfig `json:"analogs,omitempty"`
	DigitalInterrupts []board.DigitalInterruptConfig      `json:"digital_interrupts,omitempty"`
	Attributes        rdkutils.AttributeMap               `json:"attributes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (conf *Config) Validate(path string) ([]string, error) {
	for idx, c := range conf.AnalogReaders {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "analogs", idx)); err != nil {
			return nil, err
		}
	}
	for idx, c := range conf.DigitalInterrupts {
		if err := c.Validate(fmt.Sprintf("%s.%s.%d", path, "digital_interrupts", idx)); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// piPigpio is an implementation of a board.Board of a Raspberry Pi
// accessed via pigpio.
type piPigpio struct {
	resource.Named
	// To prevent deadlocks, we must never lock this mutex while instanceMu, defined below, is
	// locked. It's okay to lock instanceMu while this is locked, though. This invariant prevents
	// deadlocks if both mutexes are locked by separate goroutines and are each waiting to lock the
	// other as well.
	mu            sync.Mutex
	cancelCtx     context.Context
	cancelFunc    context.CancelFunc
	duty          int // added for mutex
	gpioConfigSet map[int]bool
	analogReaders map[string]board.AnalogReader
	// `interrupts` maps interrupt names to the interrupts. `interruptsHW` maps broadcom addresses
	// to these same values. The two should always have the same set of values.
	interrupts   map[string]board.ReconfigurableDigitalInterrupt
	interruptsHW map[uint]board.ReconfigurableDigitalInterrupt
	logger       logging.Logger
	isClosed     bool
}

var (
	pigpioInitialized bool
	// To prevent deadlocks, we must never lock the mutex of a specific piPigpio struct, above,
	// while this is locked. It is okay to lock this while one of those other mutexes is locked
	// instead.
	instanceMu sync.RWMutex
	instances  = map[*piPigpio]struct{}{}
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
func newPigpio(ctx context.Context, name resource.Name, cfg resource.Config, logger logging.Logger) (board.Board, error) {
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
		Named:      name.AsNamed(),
		logger:     logger,
		isClosed:   false,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}

	if err := piInstance.Reconfigure(ctx, nil, cfg); err != nil {
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

func (pi *piPigpio) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	if err := pi.reconfigureAnalogReaders(ctx, cfg); err != nil {
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

func (pi *piPigpio) reconfigureAnalogReaders(ctx context.Context, cfg *Config) error {
	// No need to reconfigure the old analog readers; just throw them out and make new ones.
	pi.analogReaders = map[string]board.AnalogReader{}
	for _, ac := range cfg.AnalogReaders {
		channel, err := strconv.Atoi(ac.Pin)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", ac.Pin)
		}

		bus := &piPigpioSPI{pi: pi, busSelect: ac.SPIBus}
		ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, ac.ChipSelect}

		pi.analogReaders[ac.Name] = board.SmoothAnalogReader(ar, board.AnalogReaderConfig{
			AverageOverMillis: ac.AverageOverMillis, SamplesPerSecond: ac.SamplesPerSecond,
		}, pi.logger)
	}
	return nil
}

// This is a helper function for digital interrupt reconfiguration. It finds the key in the map
// whose value is the given interrupt, and returns that key and whether we successfully found it.
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

// This is a very similar helper function, which does the same thing but for broadcom addresses.
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

func (pi *piPigpio) reconfigureInterrupts(ctx context.Context, cfg *Config) error {
	// We reuse the old interrupts when possible.
	oldInterrupts := pi.interrupts
	oldInterruptsHW := pi.interruptsHW
	// Like with pi.interrupts and pi.interruptsHW, these two will have identical values, mapped to
	// using different keys.
	newInterrupts := map[string]board.ReconfigurableDigitalInterrupt{}
	newInterruptsHW := map[uint]board.ReconfigurableDigitalInterrupt{}

	// This begins as a set of all interrupts, but we'll remove the ones we reuse. Then, we'll
	// close whatever is left over.
	interruptsToClose := make(map[board.ReconfigurableDigitalInterrupt]struct{}, len(oldInterrupts))
	for _, interrupt := range oldInterrupts {
		interruptsToClose[interrupt] = struct{}{}
	}

	reuseInterrupt := func(
		interrupt board.ReconfigurableDigitalInterrupt, name string, bcom uint,
	) error {
		newInterrupts[name] = interrupt
		newInterruptsHW[bcom] = interrupt
		delete(interruptsToClose, interrupt)

		// We also need to remove the reused interrupt from oldInterrupts and oldInterruptsHW, to
		// avoid double-reuse (e.g., the old interrupt had name "foo" on pin 7, and the new config
		// has name "foo" on pin 8 and name "bar" on pin 7).
		if oldName, ok := findInterruptName(interrupt, oldInterrupts); ok {
			delete(oldInterrupts, oldName)
		} else {
			// This should never happen. However, if it does, nothing is obviously broken, so we'll
			// just log the weirdness and continue.
			pi.logger.Errorf(
				"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
					"but couldn't find its old name!?", name, bcom)
		}

		if oldBcom, ok := findInterruptBcom(interrupt, oldInterruptsHW); ok {
			delete(oldInterruptsHW, oldBcom)
			if result := C.teardownInterrupt(C.int(oldBcom)); result != 0 {
				return picommon.ConvertErrorCodeToMessage(int(result), "error")
			}
		} else {
			// This should never happen, either, but is similarly not really a problem.
			pi.logger.Errorf(
				"Tried reconfiguring old interrupt to new name %s and broadcom address %s, "+
					"but couldn't find its old bcom!?", name, bcom)
		}

		if result := C.setupInterrupt(C.int(bcom)); result != 0 {
			return picommon.ConvertErrorCodeToMessage(int(result), "error")
		}
		return nil
	}

	for _, newConfig := range cfg.DigitalInterrupts {
		bcom, ok := broadcomPinFromHardwareLabel(newConfig.Pin)
		if !ok {
			return errors.Errorf("no hw mapping for %s", newConfig.Pin)
		}

		// Try reusing an interrupt with the same pin
		if oldInterrupt, ok := oldInterruptsHW[bcom]; ok {
			if err := reuseInterrupt(oldInterrupt, newConfig.Name, bcom); err != nil {
				return err
			}
			continue
		}
		// If that didn't work, try reusing an interrupt with the same name
		if oldInterrupt, ok := oldInterrupts[newConfig.Name]; ok {
			if err := reuseInterrupt(oldInterrupt, newConfig.Name, bcom); err != nil {
				return err
			}
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
	for interrupt := range interruptsToClose {
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
			if err := interrupt.Close(ctx); err != nil {
				return err // This should never happen, but it makes the linter happy.
			}
			if result := C.teardownInterrupt(C.int(bcom)); result != 0 {
				return picommon.ConvertErrorCodeToMessage(int(result), "error")
			}
		}
	}

	pi.interrupts = newInterrupts
	pi.interruptsHW = newInterruptsHW
	return nil
}

// GPIOPinByName returns a GPIOPin by name.
func (pi *piPigpio) GPIOPinByName(pin string) (board.GPIOPin, error) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
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
	pi.mu.Lock()
	defer pi.mu.Unlock()
	dutyCycle := rdkutils.ScaleByPct(255, dutyCyclePct)
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
	pi.mu.Lock()
	defer pi.mu.Unlock()
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

func (s *piPigpioSPI) OpenHandle() (buses.SPIHandle, error) {
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

// AnalogReaderNames returns the names of all known analog readers.
func (pi *piPigpio) AnalogReaderNames() []string {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	names := []string{}
	for k := range pi.analogReaders {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (pi *piPigpio) DigitalInterruptNames() []string {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	names := []string{}
	for k := range pi.interrupts {
		names = append(names, k)
	}
	return names
}

// AnalogReaderByName returns an analog reader by name.
func (pi *piPigpio) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	pi.mu.Lock()
	defer pi.mu.Unlock()
	a, ok := pi.analogReaders[name]
	return a, ok
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
			if result := C.setupInterrupt(C.int(bcom)); result != 0 {
				err := picommon.ConvertErrorCodeToMessage(int(result), "error")
				pi.logger.Errorf("Unable to set up interrupt on pin %s: %s", name, err)
				return nil, false
			}

			pi.interrupts[name] = d
			pi.interruptsHW[bcom] = d
			return d, true
		}
	}
	return d, ok
}

func (pi *piPigpio) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

// WriteAnalog writes the value to the given pin.
func (pi *piPigpio) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	return grpc.UnimplementedError
}

// Close attempts to close all parts of the board cleanly.
func (pi *piPigpio) Close(ctx context.Context) error {
	var terminate bool
	// Prevent duplicate calls to Close a board as this may overlap with
	// the reinitialization of the board
	pi.mu.Lock()
	defer pi.mu.Unlock()
	if pi.isClosed {
		pi.logger.Info("Duplicate call to close pi board detected, skipping")
		return nil
	}
	pi.cancelFunc()

	var err error
	for _, analog := range pi.analogReaders {
		err = multierr.Combine(err, analog.Close(ctx))
	}
	pi.analogReaders = map[string]board.AnalogReader{}

	for bcom, interrupt := range pi.interruptsHW {
		err = multierr.Combine(err, interrupt.Close(ctx))
		if result := C.teardownInterrupt(C.int(bcom)); result != 0 {
			err = multierr.Combine(err, picommon.ConvertErrorCodeToMessage(int(result), "error"))
		}
	}
	pi.interrupts = map[string]board.ReconfigurableDigitalInterrupt{}
	pi.interruptsHW = map[uint]board.ReconfigurableDigitalInterrupt{}

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
			logging.Global().Infof("no DigitalInterrupt configured for gpio %d", gpio)
			continue
		}
		high := true
		if level == 0 {
			high = false
		}
		// this should *not* block for long otherwise the lock
		// will be held
		err := i.Tick(instance.cancelCtx, high, tick*1000)
		if err != nil {
			instance.logger.Error(err)
		}
	}
}
