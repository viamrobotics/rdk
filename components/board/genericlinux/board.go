//go:build linux

// Package genericlinux implements a Linux-based board making heavy use of sysfs
// (https://en.wikipedia.org/wiki/Sysfs). This does not provide a board model itself but provides
// the underlying logic for any Linux/sysfs based board.
package genericlinux

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/resource"
)

// RegisterBoard registers a sysfs based board of the given model.
func RegisterBoard(modelName string, gpioMappings map[string]GPIOBoardMapping) {
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
				return NewBoard(ctx, conf, ConstPinDefs(gpioMappings), logger)
			},
		})
}

// NewBoard is the constructor for a Board.
func NewBoard(
	ctx context.Context,
	conf resource.Config,
	convertConfig ConfigConverter,
	logger golog.Logger,
) (board.Board, error) {
	newConf, err := convertConfig(conf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	b := &Board{
		Named:         conf.ResourceName().AsNamed(),
		convertConfig: convertConfig,

		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,

		spis:       map[string]*spiBus{},
		analogs:    map[string]*wrappedAnalog{},
		i2cs:       map[string]*I2cBus{},
		gpios:      map[string]*gpioPin{},
		interrupts: map[string]*digitalInterrupt{},
	}

	// TODO(RSDK_4092): Move this part into reconfiguration.
	for pinName, mapping := range newConf.GpioMappings {
		b.gpios[pinName] = b.createGpioPin(mapping)
	}

	if err := b.Reconfigure(ctx, nil, conf); err != nil {
		return nil, err
	}
	return b, nil
}

// Reconfigure reconfigures the board with interrupt pins, spi and i2c, and analogs.
func (b *Board) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	newConf, err := b.convertConfig(conf)
	if err != nil {
		return err
	}

	return b.ReconfigureParsedConfig(ctx, *newConf)
}

// ReconfigureParsedConfig is a public helper that should only be used
// by the customlinux package.
func (b *Board) ReconfigureParsedConfig(ctx context.Context, conf LinuxBoardConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.reconfigureGpios(conf); err != nil {
		return err
	}
	if err := b.reconfigureSpis(conf); err != nil {
		return err
	}
	if err := b.reconfigureI2cs(conf); err != nil {
		return err
	}
	if err := b.reconfigureAnalogs(ctx, conf); err != nil {
		return err
	}
	if err := b.reconfigureInterrupts(conf); err != nil {
		return err
	}
	return nil
}

func (b *Board) reconfigureGpios(newConf LinuxBoardConfig) error {
	// TODO(RSDK-4092): implement this correctly.
	if len(b.gpioMappings) == 0 {
		b.gpioMappings = newConf.GpioMappings
	}
	return nil
}

// This never returns errors, but we give it the same function signature as the other
// reconfiguration helpers for consistency.
func (b *Board) reconfigureSpis(newConf LinuxBoardConfig) error {
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

func (b *Board) reconfigureI2cs(newConf LinuxBoardConfig) error {
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
		bus, err := NewI2cBus(c.Bus)
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

func (b *Board) reconfigureAnalogs(ctx context.Context, newConf LinuxBoardConfig) error {
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

// This helper function is used while reconfiguring digital interrupts. It finds the new config (if
// any) for a pre-existing digital interrupt.
func findNewDigIntConfig(
	interrupt *digitalInterrupt, confs []board.DigitalInterruptConfig, logger golog.Logger,
) *board.DigitalInterruptConfig {
	for _, newConfig := range confs {
		if newConfig.Pin == interrupt.config.Pin {
			return &newConfig
		}
	}
	if interrupt.config.Name == interrupt.config.Pin {
		// This interrupt is named identically to its pin. It was probably created on the fly
		// by some other component (an encoder?). Unless there's now some other config with the
		// same name but on a different pin, keep it initialized as-is.
		for _, intConfig := range confs {
			if intConfig.Name == interrupt.config.Name {
				// The name of this interrupt is defined in the new config, but on a different
				// pin. This interrupt should be closed.
				return nil
			}
		}
		logger.Debugf(
			"Keeping digital interrupt on pin %s even though it's not explicitly mentioned "+
				"in the new board config",
			interrupt.config.Pin)
		return interrupt.config
	}
	return nil
}

func (b *Board) reconfigureInterrupts(newConf LinuxBoardConfig) error {
	// Any pin that already exists in the right configuration should just be copied over; closing
	// and re-opening it risks losing its state.
	newInterrupts := make(map[string]*digitalInterrupt, len(newConf.DigitalInterrupts))

	// Reuse any old interrupts that have new configs
	for _, oldInterrupt := range b.interrupts {
		if newConfig := findNewDigIntConfig(oldInterrupt, newConf.DigitalInterrupts, b.logger); newConfig == nil {
			// The old interrupt shouldn't exist any more, but it probably became a GPIO pin.
			if err := oldInterrupt.Close(); err != nil {
				return err // This should never happen, but the linter worries anyway.
			}
			if newGpioConfig, ok := b.gpioMappings[oldInterrupt.config.Pin]; ok {
				// See gpio.go for createGpioPin.
				b.gpios[oldInterrupt.config.Pin] = b.createGpioPin(newGpioConfig)
			} else {
				b.logger.Warnf("Old interrupt pin was on nonexistent GPIO pin '%s', ignoring",
					oldInterrupt.config.Pin)
			}
		} else { // The old interrupt should stick around.
			if err := oldInterrupt.interrupt.Reconfigure(*newConfig); err != nil {
				return err
			}
			oldInterrupt.config = newConfig
			newInterrupts[newConfig.Name] = oldInterrupt
		}
	}
	oldInterrupts := b.interrupts
	b.interrupts = newInterrupts

	// Add any new interrupts that should be freshly made.
	for _, config := range newConf.DigitalInterrupts {
		if interrupt, ok := b.interrupts[config.Name]; ok {
			if interrupt.config.Pin == config.Pin {
				continue // Already initialized; keep going
			}
			// If the interrupt's name matches but the pin does not, the interrupt we already have
			// was implicitly created (e.g., its name is "38" so we created it on pin 38 even
			// though it was not explicitly mentioned in the old board config), but the new config
			// is explicit (e.g., its name is still "38" but it's been moved to pin 37). Close the
			// old one and initialize it anew.
			if err := interrupt.Close(); err != nil {
				return err
			}
			// Although we delete the implicit interrupt from b.interrupts, it's still in
			// oldInterrupts, so we haven't lost the channels it reports to and can still copy them
			// over to the new struct.
			delete(b.interrupts, config.Name)
		}

		if oldPin, ok := b.gpios[config.Pin]; ok {
			if err := oldPin.Close(); err != nil {
				return err
			}
			delete(b.gpios, config.Pin)
		}

		// If there was an old interrupt pin with this same name, reuse the part that holds its
		// callbacks. Anything subscribed to the old pin will expect to still be subscribed to the
		// new one.
		var oldCallbackHolder board.ReconfigurableDigitalInterrupt
		if oldInterrupt, ok := oldInterrupts[config.Name]; ok {
			oldCallbackHolder = oldInterrupt.interrupt
		}
		interrupt, err := b.createDigitalInterrupt(
			b.cancelCtx, config, b.gpioMappings, oldCallbackHolder)
		if err != nil {
			return err
		}
		b.interrupts[config.Name] = interrupt
	}

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

// Board implements a component for a Linux machine.
type Board struct {
	resource.Named
	mu            sync.RWMutex
	convertConfig ConfigConverter

	gpioMappings map[string]GPIOBoardMapping
	spis         map[string]*spiBus
	analogs      map[string]*wrappedAnalog
	i2cs         map[string]*I2cBus
	logger       golog.Logger

	gpios      map[string]*gpioPin
	interrupts map[string]*digitalInterrupt

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// SPIByName returns the SPI by the given name if it exists.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	s, ok := b.spis[name]
	return s, ok
}

// I2CByName returns the i2c by the given name if it exists.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	i, ok := b.i2cs[name]
	return i, ok
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogs[name]
	return a, ok
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
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
	interrupt, err := b.createDigitalInterrupt(
		b.cancelCtx, defaultInterruptConfig, b.gpioMappings, nil)
	if err != nil {
		b.logger.Errorw("failed to create digital interrupt pin on the fly", "error", err)
		return nil, false
	}

	delete(b.gpios, name)
	b.interrupts[name] = interrupt
	return interrupt.interrupt, true
}

// SPINames returns the names of all known SPIs.
func (b *Board) SPINames() []string {
	if len(b.spis) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.spis))
	for k := range b.spis {
		names = append(names, k)
	}
	return names
}

// I2CNames returns the names of all known I2Cs.
func (b *Board) I2CNames() []string {
	if len(b.i2cs) == 0 {
		return nil
	}
	names := make([]string, 0, len(b.i2cs))
	for k := range b.i2cs {
		names = append(names, k)
	}
	return names
}

// AnalogReaderNames returns the names of all known analog readers.
func (b *Board) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogs {
		names = append(names, k)
	}
	return names
}

// DigitalInterruptNames returns the names of all known digital interrupts.
func (b *Board) DigitalInterruptNames() []string {
	if b.interrupts == nil {
		return nil
	}

	names := []string{}
	for name := range b.interrupts {
		names = append(names, name)
	}
	return names
}

// GPIOPinNames returns the names of all known GPIO pins.
func (b *Board) GPIOPinNames() []string {
	if b.gpioMappings == nil {
		return nil
	}
	names := []string{}
	for k := range b.gpioMappings {
		names = append(names, k)
	}
	return names
}

// GPIOPinByName returns a GPIOPin by name.
func (b *Board) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	if pin, ok := b.gpios[pinName]; ok {
		return pin, nil
	}

	// Check if pin is a digital interrupt: those can still be used as inputs.
	if interrupt, interruptOk := b.interrupts[pinName]; interruptOk {
		return &gpioInterruptWrapperPin{*interrupt}, nil
	}

	return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", pinName)
}

// Status returns the current status of the board.
func (b *Board) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	return &commonpb.BoardStatus{}, nil
}

// ModelAttributes returns attributes related to the model of this board.
func (b *Board) ModelAttributes() board.ModelAttributes {
	return board.ModelAttributes{}
}

// SetPowerMode sets the board to the given power mode. If provided,
// the board will exit the given power mode after the specified
// duration.
func (b *Board) SetPowerMode(
	ctx context.Context,
	mode pb.PowerMode,
	duration *time.Duration,
) error {
	return grpc.UnimplementedError
}

// Close attempts to cleanly close each part of the board.
func (b *Board) Close(ctx context.Context) error {
	b.mu.Lock()
	b.cancelFunc()
	b.mu.Unlock()
	b.activeBackgroundWorkers.Wait()

	var err error
	for _, pin := range b.gpios {
		err = multierr.Combine(err, pin.Close())
	}
	for _, interrupt := range b.interrupts {
		err = multierr.Combine(err, interrupt.Close())
	}
	return err
}
