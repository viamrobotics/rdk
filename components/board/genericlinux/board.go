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
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
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
				logger logging.Logger,
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
	logger logging.Logger,
) (board.Board, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	b := &Board{
		Named:         conf.ResourceName().AsNamed(),
		convertConfig: convertConfig,

		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,

		spis:          map[string]*spiBus{},
		analogReaders: map[string]*wrappedAnalogReader{},
		gpios:         map[string]*gpioPin{},
		interrupts:    map[string]*digitalInterrupt{},
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

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.reconfigureGpios(newConf); err != nil {
		return err
	}
	if err := b.reconfigureSpis(newConf); err != nil {
		return err
	}
	if err := b.reconfigureAnalogReaders(ctx, newConf); err != nil {
		return err
	}
	if err := b.reconfigureInterrupts(newConf); err != nil {
		return err
	}
	return nil
}

// This is a helper function used to reconfigure the GPIO pins. It looks for the key in the map
// whose value resembles the target pin definition.
func getMatchingPin(target GPIOBoardMapping, mapping map[string]GPIOBoardMapping) (string, bool) {
	for name, def := range mapping {
		if target == def {
			return name, true
		}
	}
	return "", false
}

func (b *Board) reconfigureGpios(newConf *LinuxBoardConfig) error {
	// First, find old pins that are no longer defined, and destroy them.
	for oldName, mapping := range b.gpioMappings {
		if _, ok := getMatchingPin(mapping, newConf.GpioMappings); ok {
			continue // This pin is in the new mapping, so don't destroy it.
		}

		// Otherwise, remove the pin because it's not in the new mapping.
		if pin, ok := b.gpios[oldName]; ok {
			if err := pin.Close(); err != nil {
				return err
			}
			delete(b.gpios, oldName)
			continue
		}

		// If we get here, the old pin definition exists, but the old pin does not. Check if it's a
		// digital interrupt.
		if interrupt, ok := b.interrupts[oldName]; ok {
			if err := interrupt.Close(); err != nil {
				return err
			}
			delete(b.interrupts, oldName)
			continue
		}

		// If we get here, there is a logic bug somewhere. but failing to delete a nonexistent pin
		// seemingly doesn't hurt anything, so just log the error and continue.
		b.logger.Errorf("During reconfiguration, old pin '%s' should be destroyed, but "+
			"it doesn't exist!?", oldName)
	}

	// Next, compare the new pin definitions to the old ones, to build up 2 sets: pins to rename,
	// and new pins to create. Don't actually create any yet, in case you'd overwrite a pin that
	// should be renamed out of the way first.
	toRename := map[string]string{} // Maps old names for pins to new names
	toCreate := map[string]GPIOBoardMapping{}
	for newName, mapping := range newConf.GpioMappings {
		if oldName, ok := getMatchingPin(mapping, b.gpioMappings); ok {
			if oldName != newName {
				toRename[oldName] = newName
			}
		} else {
			toCreate[newName] = mapping
		}
	}

	// Rename the ones whose name changed. The ordering here is tricky: if B should be renamed to C
	// while A should be renamed to B, we need to make sure we don't overwrite B with A and then
	// rename it to C. To avoid this, move all the pins to rename into a temporary data structure,
	// then move them all back again afterward.
	tempGpios := map[string]*gpioPin{}
	tempInterrupts := map[string]*digitalInterrupt{}
	for oldName, newName := range toRename {
		if pin, ok := b.gpios[oldName]; ok {
			tempGpios[newName] = pin
			delete(b.gpios, oldName)
			continue
		}

		// If we get here, again check if the missing pin is a digital interrupt.
		if interrupt, ok := b.interrupts[oldName]; ok {
			tempInterrupts[newName] = interrupt
			delete(b.interrupts, oldName)
			continue
		}

		return fmt.Errorf("during reconfiguration, old pin '%s' should be renamed to '%s', but "+
			"it doesn't exist!?", oldName, newName)
	}

	// Now move all the pins back from the temporary data structures.
	for newName, pin := range tempGpios {
		b.gpios[newName] = pin
	}
	for newName, interrupt := range tempInterrupts {
		b.interrupts[newName] = interrupt
	}

	// Finally, create the new pins.
	for newName, mapping := range toCreate {
		b.gpios[newName] = b.createGpioPin(mapping)
	}

	b.gpioMappings = newConf.GpioMappings
	return nil
}

// This never returns errors, but we give it the same function signature as the other
// reconfiguration helpers for consistency.
func (b *Board) reconfigureSpis(newConf *LinuxBoardConfig) error {
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

func (b *Board) reconfigureAnalogReaders(ctx context.Context, newConf *LinuxBoardConfig) error {
	stillExists := map[string]struct{}{}
	for _, c := range newConf.AnalogReaders {
		channel, err := strconv.Atoi(c.Pin)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", c.Pin)
		}

		bus := &spiBus{}
		bus.reset(c.SPIBus)

		stillExists[c.Name] = struct{}{}
		if curr, ok := b.analogReaders[c.Name]; ok {
			if curr.chipSelect != c.ChipSelect {
				ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, c.ChipSelect}
				curr.reset(ctx, curr.chipSelect,
					board.SmoothAnalogReader(ar, board.AnalogReaderConfig{
						AverageOverMillis: c.AverageOverMillis, SamplesPerSecond: c.SamplesPerSecond,
					}, b.logger))
			}
			continue
		}
		ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, c.ChipSelect}
		b.analogReaders[c.Name] = newWrappedAnalogReader(ctx, c.ChipSelect,
			board.SmoothAnalogReader(ar, board.AnalogReaderConfig{
				AverageOverMillis: c.AverageOverMillis, SamplesPerSecond: c.SamplesPerSecond,
			}, b.logger))
	}

	for name := range b.analogReaders {
		if _, ok := stillExists[name]; ok {
			continue
		}
		b.analogReaders[name].reset(ctx, "", nil)
		delete(b.analogReaders, name)
	}
	return nil
}

// This helper function is used while reconfiguring digital interrupts. It finds the new config (if
// any) for a pre-existing digital interrupt.
func findNewDigIntConfig(
	interrupt *digitalInterrupt, confs []board.DigitalInterruptConfig, logger logging.Logger,
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

func (b *Board) reconfigureInterrupts(newConf *LinuxBoardConfig) error {
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

type wrappedAnalogReader struct {
	mu         sync.RWMutex
	chipSelect string
	reader     *board.AnalogSmoother
}

func newWrappedAnalogReader(ctx context.Context, chipSelect string, reader *board.AnalogSmoother) *wrappedAnalogReader {
	var wrapped wrappedAnalogReader
	wrapped.reset(ctx, chipSelect, reader)
	return &wrapped
}

func (a *wrappedAnalogReader) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.reader == nil {
		return 0, errors.New("closed")
	}
	return a.reader.Read(ctx, extra)
}

func (a *wrappedAnalogReader) Close(ctx context.Context) error {
	return nil
}

func (a *wrappedAnalogReader) reset(ctx context.Context, chipSelect string, reader *board.AnalogSmoother) {
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

	gpioMappings  map[string]GPIOBoardMapping
	spis          map[string]*spiBus
	analogReaders map[string]*wrappedAnalogReader
	logger        logging.Logger

	gpios      map[string]*gpioPin
	interrupts map[string]*digitalInterrupt

	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

// AnalogReaderByName returns the analog reader by the given name if it exists.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	a, ok := b.analogReaders[name]
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

// AnalogReaderNames returns the names of all known analog readers.
func (b *Board) AnalogReaderNames() []string {
	names := []string{}
	for k := range b.analogReaders {
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
	return board.CreateStatus(ctx, b, extra)
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

// WriteAnalog writes the value to the given pin.
func (b *Board) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	return nil
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
