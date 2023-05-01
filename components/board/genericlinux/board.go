//go:build linux

// Package genericlinux implements a Linux-based board making heavy use of sysfs
// (https://en.wikipedia.org/wiki/Sysfs). This does not provide a board model itself but provides
// the underlying logic for any Linux/sysfs based board.
package genericlinux

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	goutils "go.viam.com/utils"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
)

// RegisterBoard registers a sysfs based board of the given model.
func RegisterBoard(modelName string, gpioMappings map[int]GPIOBoardMapping, usePeriphGpio bool) {
	resource.RegisterComponent(
		board.API,
		resource.DefaultModelFamily.WithModel(modelName),
		resource.Registration[board.Board, *Config]{
			Constructor: func(
				ctx context.Context,
				_ resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (board.Board, error) {
				return newBoard(ctx, conf, gpioMappings, usePeriphGpio, logger)
			},
		})
}

func newBoard(
	ctx context.Context,
	conf resource.Config,
	gpioMappings map[int]GPIOBoardMapping,
	usePeriphGpio bool,
	logger golog.Logger,
) (board.Board, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	b := sysfsBoard{
		Named:         conf.ResourceName().AsNamed(),
		usePeriphGpio: usePeriphGpio,
		gpioMappings:  gpioMappings,
		logger:        logger,
		cancelCtx:     cancelCtx,
		cancelFunc:    cancelFunc,

		spis:    map[string]*spiBus{},
		analogs: map[string]*wrappedAnalog{},
		// this is not yet modified during reconfiguration but maybe should be
		pwms:       map[string]pwmSetting{},
		i2cs:       map[string]*i2cBus{},
		gpios:      map[string]*gpioPin{},
		interrupts: map[string]*digitalInterrupt{},
	}

	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *sysfsBoard) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if err := b.reconfigureSpis(newConf); err != nil {
		return err
	}

	if err := b.reconfigureI2cs(newConf); err != nil {
		return err
	}

	if err := b.reconfigureAnalogs(ctx, newConf); err != nil {
		return err
	}

	if err := b.reconfigureGpios(newConf); err != nil {
		return err
	}

	return nil
}

// This never returns errors, but we give it the same function signature as the other
// reconfiguration helpers for consistency.
func (b *sysfsBoard) reconfigureSpis(newConf *Config) error {
	stillExists := map[string]struct{}{}
	for _, c := range newConf.SPIs {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.spis[c.Name]; ok {
			if busPtr := curr.bus.Load(); busPtr != nil && *busPtr != c.BusSelect {
				curr.reset(c.BusSelect)
			}
			continue
		}
		b.spis[c.Name] = &spiBus{}
		b.spis[c.Name].reset(c.BusSelect)
	}

	for name := range b.spis {
		if _, ok := stillExists[name]; ok {
			continue
		}
		delete(b.spis, name)
	}
	return nil
}

func (b *sysfsBoard) reconfigureI2cs(newConf *Config) error {
	stillExists := map[string]struct{}{}
	for _, c := range newConf.I2Cs {
		stillExists[c.Name] = struct{}{}
		if curr, ok := b.i2cs[c.Name]; ok {
			if curr.deviceName == c.Bus {
				continue
			}
			if err := curr.closeableBus.Close(); err != nil {
				b.logger.Errorw("error closing I2C bus while reconfiguring", "error", err)
			}
			if err := curr.reset(curr.deviceName); err != nil {
				b.logger.Errorw("error resetting I2C bus while reconfiguring", "error", err)
			}
			continue
		}
		bus, err := newI2cBus(c.Bus)
		if err != nil {
			return err
		}
		b.i2cs[c.Name] = bus
	}

	for name := range b.i2cs {
		if _, ok := stillExists[name]; ok {
			continue
		}
		if err := b.i2cs[name].closeableBus.Close(); err != nil {
			b.logger.Errorw("error closing I2C bus while reconfiguring", "error", err)
		}
		delete(b.i2cs, name)
	}
	return nil
}

func (b *sysfsBoard) reconfigureAnalogs(ctx context.Context, newConf *Config) error {
	stillExists := map[string]struct{}{}
	for _, c := range newConf.Analogs {
		channel, err := strconv.Atoi(c.Pin)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", c.Pin)
		}

		bus, ok := b.spis[c.SPIBus]
		if !ok {
			return errors.Errorf("can't find SPI bus (%s) requested by AnalogReader", c.SPIBus)
		}

		stillExists[c.Name] = struct{}{}
		if curr, ok := b.analogs[c.Name]; ok {
			if curr.chipSelect != c.ChipSelect {
				ar := &board.MCP3008AnalogReader{channel, bus, c.ChipSelect}
				curr.reset(ctx, curr.chipSelect, board.SmoothAnalogReader(ar, c, b.logger))
			}
			continue
		}
		ar := &board.MCP3008AnalogReader{channel, bus, c.ChipSelect}
		b.analogs[c.Name] = newWrappedAnalog(ctx, c.ChipSelect, board.SmoothAnalogReader(ar, c, b.logger))
	}

	for name := range b.analogs {
		if _, ok := stillExists[name]; ok {
			continue
		}
		b.analogs[name].reset(ctx, "", nil)
		delete(b.analogs, name)
	}
	return nil
}

func (b *sysfsBoard) reconfigureGpios(newConf *Config) error {
	if b.usePeriphGpio {
		if len(newConf.DigitalInterrupts) != 0 {
			return errors.New("digital interrupts on Periph GPIO pins are not yet supported")
		}
		return nil // No digital interrupts to reconfigure.
	}

	// If we get here, we need to reconfigure b.gpios and b.interrupts. Any pin that already exists
	// in the right configuration should just be copied over; closing and re-opening it risks
	// losing its state.
	newInterrupts := make(map[string]*digitalInterrupt, len(newConf.DigitalInterrupts))
	newGpios := make(map[string]*gpioPin)

	// Here's a helper function, which finds the new config for a pre-existing digital interrupt.
	findNewDigIntConfig := func(interrupt *digitalInterrupt) *board.DigitalInterruptConfig {
		for _, newConfig := range newConf.DigitalInterrupts {
			if newConfig.Pin == interrupt.config.Pin {
				return &newConfig
			}
		}
		if interrupt.config.Name == interrupt.config.Pin {
			// This interrupt is named identically to its pin. It was probably created on the fly
			// by some other component (an encoder?). Keep it initialized as-is, even though it's
			// not explicitly mentioned in the config, because it's probably still in-use.
			b.logger.Debugf(
				"Keeping digital interrupt on pin %s even though it's not explicitly mentioned " +
				"in the new board config",
				interrupt.config.Pin)
			return interrupt.config
		}
		return nil
	}

	// Reuse any old interrupts that should stick around.
	for _, oldInterrupt := range b.interrupts {
		if newConfig := findNewDigIntConfig(oldInterrupt); newConfig == nil {
			// The old interrupt shouldn't exist any more.
			oldInterrupt.Close()
		} else {
			newInterrupts[newConfig.Name] = oldInterrupt
			if err := oldInterrupt.interrupt.Reconfigure(*newConfig); err != nil {
				return err
			}
		}
	}
	b.interrupts = newInterrupts

	// Reuse any old GPIO pins that should stick around, too.
	for pin, oldGpio := range b.gpios {
		// TODO
	}

	// Add any new interrupts that should be freshly made.
	// TODO

	// Finally, add any new GPIO pins.
	// TODO

	/*
	for each old interrupt:
	    if it's either numerically named or in the new config, copy it over to the new map and reconfigure
		else close it
	for each old GPIO pin:
	    if it's in the new config and it's not an interrupt, copy it over
		else close it
	for each new interrupt:
	    if it doesn't exist yet, close the old GPIO pin and then create it
	for each GPIO pin in the config:
	    if it's an interrupt, skip it
		create it
	*/

	// TODO(RSDK-2684): we dont configure pins so we just unset them here. not really great behavior.
	// We currently have two implementations of GPIO pins on these boards: one using
	// libraries from periph.io and one using an ioctl approach. If we're using the
	// latter, we need to initialize it here.
	gpios, interrupts, err := b.gpioInitialize( // Defined in gpio.go
		b.cancelCtx,
		b.gpioMappings,
		newConf.DigitalInterrupts,
		b.logger)
	if err != nil {
		return err
	}
	b.gpios = gpios
	b.interrupts = interrupts
	return nil
}

type wrappedAnalog struct {
	mu         sync.RWMutex
	chipSelect string
	reader     *board.AnalogSmoother
}

func newWrappedAnalog(ctx context.Context, chipSelect string, reader *board.AnalogSmoother) *wrappedAnalog {
	var wrapped wrappedAnalog
	wrapped.reset(ctx, chipSelect, reader)
	return &wrapped
}

func (a *wrappedAnalog) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.reader == nil {
		return 0, errors.New("closed")
	}
	return a.reader.Read(ctx, extra)
}

func (a *wrappedAnalog) Close(ctx context.Context) error {
	return nil
}

func (a *wrappedAnalog) reset(ctx context.Context, chipSelect string, reader *board.AnalogSmoother) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.reader != nil {
		goutils.UncheckedError(a.reader.Close(ctx))
	}
	a.reader = reader
	a.chipSelect = chipSelect
}

type sysfsBoard struct {
	resource.Named
	mu           sync.RWMutex
	gpioMappings map[int]GPIOBoardMapping
	spis         map[string]*spiBus
	analogs      map[string]*wrappedAnalog
	pwms         map[string]pwmSetting
	i2cs         map[string]*i2cBus
	logger       golog.Logger

	usePeriphGpio bool
	// These next two are only used for non-periph.io pins
	gpios      map[string]*gpioPin
	interrupts map[string]*digitalInterrupt

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

type pwmSetting struct {
	dutyCycle gpio.Duty
	frequency physic.Frequency
}

func (b *sysfsBoard) SPIByName(name string) (board.SPI, bool) {
	s, ok := b.spis[name]
	return s, ok
}

type spiBus struct {
	mu         sync.Mutex
	openHandle *spiHandle
	bus        atomic.Pointer[string]
}

type spiHandle struct {
	bus      *spiBus
	isClosed bool
}

func (sb *spiBus) OpenHandle() (board.SPIHandle, error) {
	sb.mu.Lock()
	sb.openHandle = &spiHandle{bus: sb, isClosed: false}
	return sb.openHandle, nil
}

func (sb *spiBus) Close(ctx context.Context) error {
	return nil
}

func (sb *spiBus) reset(bus string) {
	sb.bus.Store(&bus)
}

func (sh *spiHandle) Xfer(ctx context.Context, baud uint, chipSelect string, mode uint, tx []byte) (rx []byte, err error) {
	if sh.isClosed {
		return nil, errors.New("can't use Xfer() on an already closed SPIHandle")
	}

	busPtr := sh.bus.bus.Load()
	if busPtr == nil {
		return nil, errors.New("no bus selected")
	}

	port, err := spireg.Open(fmt.Sprintf("SPI%s.%s", *busPtr, chipSelect))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = multierr.Combine(err, port.Close())
	}()
	conn, err := port.Connect(physic.Hertz*physic.Frequency(baud), spi.Mode(mode), 8)
	if err != nil {
		return nil, err
	}
	rx = make([]byte, len(tx))
	return rx, conn.Tx(tx, rx)
}

func (sh *spiHandle) Close() error {
	sh.isClosed = true
	sh.bus.mu.Unlock()
	return nil
}

func (b *sysfsBoard) I2CByName(name string) (board.I2C, bool) {
	i, ok := b.i2cs[name]
	return i, ok
}

func (b *sysfsBoard) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

func (b *sysfsBoard) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	if b.usePeriphGpio {
		return nil, false // Digital interrupts aren't supported.
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	interrupt, ok := b.interrupts[name]
	if ok {
		return interrupt.interrupt, true
	}

	// Otherwise, the name is not something we recognize yet. If it appears to be a GPIO pin, we'll
	// remove its GPIO capabilities and turn it into a digital interrupt.
	gpio, ok := b.gpios[name]
	if !ok {
		return nil, false
	}
	if err := gpio.Close(); err != nil {
		b.logger.Errorw("failed to close GPIO pin to use as interrupt", "error", err)
		return nil, false
	}

	const defaultInterruptType = "basic"
	defaultInterruptConfig := board.DigitalInterruptConfig{
		Name: name,
		Pin:  name,
		Type: defaultInterruptType,
	}
	interrupt, err := b.createDigitalInterrupt(b.cancelCtx, defaultInterruptConfig, b.gpioMappings)
	if err != nil {
		b.logger.Errorw("failed to create digital interrupt pin on the fly", "error", err)
		return nil, false
	}

	delete(b.gpios, name)
	b.interrupts[name] = interrupt
	return interrupt.interrupt, true
}

func (b *sysfsBoard) SPINames() []string {
	if len(b.spis) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.spis))
	for k := range b.spis {
		names = append(names, k)
	}
	return names
}

func (b *sysfsBoard) I2CNames() []string {
	if len(b.i2cs) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.i2cs))
	for k := range b.i2cs {
		names = append(names, k)
	}
	return names
}

func (b *sysfsBoard) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogs {
		names = append(names, k)
	}
	return names
}

func (b *sysfsBoard) DigitalInterruptNames() []string {
	if b.interrupts == nil {
		return nil
	}

	names := []string{}
	for name := range b.interrupts {
		names = append(names, name)
	}
	return names
}

func (b *sysfsBoard) GPIOPinNames() []string {
	if b.gpioMappings == nil {
		return nil
	}
	names := []string{}
	for k := range b.gpioMappings {
		names = append(names, fmt.Sprintf("%d", k))
	}
	return names
}

func (b *sysfsBoard) getGPIOLine(hwPin string) (gpio.PinIO, bool, error) {
	pinName := hwPin
	hwPWMSupported := true
	if b.gpioMappings != nil {
		pinParsed, err := strconv.ParseInt(hwPin, 10, 32)
		if err != nil {
			return nil, false, errors.New("pin cannot be parsed or unset")
		}

		mapping, ok := b.gpioMappings[int(pinParsed)]
		if !ok {
			return nil, false, errors.Errorf("invalid pin \"%d\"", pinParsed)
		}
		pinName = fmt.Sprintf("%d", mapping.GPIOGlobal)
		hwPWMSupported = mapping.HWPWMSupported
	}

	pin := gpioreg.ByName(pinName)
	if pin == nil {
		return nil, false, errors.Errorf("no global pin found for %q", pinName)
	}
	return pin, hwPWMSupported, nil
}

type periphGpioPin struct {
	b              *sysfsBoard
	pin            gpio.PinIO
	pinName        string
	hwPWMSupported bool
}

func (b *sysfsBoard) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	if b.usePeriphGpio {
		return b.periphGPIOPinByName(pinName)
	}
	// Otherwise, the pins are stored in b.gpios.
	pin, ok := b.gpios[pinName]
	if !ok {
		return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", pinName)
	}
	return pin, nil
}

func (b *sysfsBoard) periphGPIOPinByName(pinName string) (board.GPIOPin, error) {
	pin, hwPWMSupported, err := b.getGPIOLine(pinName)
	if err != nil {
		return nil, err
	}

	return periphGpioPin{b, pin, pinName, hwPWMSupported}, nil
}

func (gp periphGpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	delete(gp.b.pwms, gp.pinName)

	return gp.set(high)
}

// This function is separate from Set(), above, because this one does not remove the pin from the
// board's pwms map. When simulating PWM in software, we use this function to turn the pin on and
// off while continuing to treat it as a PWM pin.
func (gp periphGpioPin) set(high bool) error {
	l := gpio.Low
	if high {
		l = gpio.High
	}
	return gp.pin.Out(l)
}

func (gp periphGpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return gp.pin.Read() == gpio.High, nil
}

func (gp periphGpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	pwm, ok := gp.b.pwms[gp.pinName]
	if !ok {
		return 0, fmt.Errorf("missing pin %s", gp.pinName)
	}
	return float64(pwm.dutyCycle) / float64(gpio.DutyMax), nil
}

// expects to already have lock acquired.
func (b *sysfsBoard) startSoftwarePWMLoop(gp periphGpioPin) {
	b.activeBackgroundWorkers.Add(1)
	goutils.ManagedGo(func() {
		b.softwarePWMLoop(b.cancelCtx, gp)
	}, b.activeBackgroundWorkers.Done)
}

func (b *sysfsBoard) softwarePWMLoop(ctx context.Context, gp periphGpioPin) {
	for {
		cont := func() bool {
			b.mu.RLock()
			defer b.mu.RUnlock()
			pwmSetting, ok := b.pwms[gp.pinName]
			if !ok {
				b.logger.Debug("pwm setting deleted; stopping")
				return false
			}

			if err := gp.set(true); err != nil {
				b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
				return true
			}
			onPeriod := time.Duration(
				int64((float64(pwmSetting.dutyCycle) / float64(gpio.DutyMax)) * float64(pwmSetting.frequency.Period())),
			)
			if !goutils.SelectContextOrWait(ctx, onPeriod) {
				return false
			}
			if err := gp.set(false); err != nil {
				b.logger.Errorw("error setting pin", "pin_name", gp.pinName, "error", err)
				return true
			}
			offPeriod := pwmSetting.frequency.Period() - onPeriod

			return goutils.SelectContextOrWait(ctx, offPeriod)
		}()
		if !cont {
			return
		}
	}
}

func (gp periphGpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var freqHz physic.Frequency
	if last.frequency != 0 {
		freqHz = last.frequency
	}
	duty := gpio.Duty(dutyCyclePct * float64(gpio.DutyMax))
	last.dutyCycle = duty
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		err := gp.pin.PWM(duty, freqHz)
		// TODO: [RSDK-569] (rh) find or implement a PWM sysfs that works with hardware pwm mappings
		// periph.io does not implement PWM
		if err != nil {
			return errors.New("sysfs PWM not currently supported, use another pin for software PWM loops")
		}
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (gp periphGpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	gp.b.mu.RLock()
	defer gp.b.mu.RUnlock()

	return uint(gp.b.pwms[gp.pinName].frequency / physic.Hertz), nil
}

func (gp periphGpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	gp.b.mu.Lock()
	defer gp.b.mu.Unlock()

	last, alreadySet := gp.b.pwms[gp.pinName]
	var duty gpio.Duty
	if last.dutyCycle != 0 {
		duty = last.dutyCycle
	}
	frequency := physic.Hertz * physic.Frequency(freqHz)
	last.frequency = frequency
	gp.b.pwms[gp.pinName] = last

	if gp.hwPWMSupported {
		return gp.pin.PWM(duty, frequency)
	}

	if !alreadySet {
		gp.b.startSoftwarePWMLoop(gp)
	}

	return nil
}

func (b *sysfsBoard) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

func (b *sysfsBoard) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

func (b *sysfsBoard) SetPowerMode(ctx context.Context, mode pb.PowerMode, duration *time.Duration) error {
	return grpc.UnimplementedError
}

func (b *sysfsBoard) Close(ctx context.Context) error {
	b.mu.Lock()
	b.cancelFunc()
	b.mu.Unlock()
	b.activeBackgroundWorkers.Wait()

	// For non-Periph boards, shut down all our open pins so we don't leak file descriptors
	if b.usePeriphGpio {
		return nil
	}

	var err error
	for _, pin := range b.gpios {
		err = multierr.Combine(err, pin.Close())
	}
	for _, interrupt := range b.interrupts {
		err = multierr.Combine(err, interrupt.Close())
	}
	return err
}
