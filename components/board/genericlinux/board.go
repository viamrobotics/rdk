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
	pb "go.viam.com/api/component/board/v1"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/board/mcp3008helper"
	"go.viam.com/rdk/components/board/pinwrappers"
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
	b := &Board{
		Named:         conf.ResourceName().AsNamed(),
		convertConfig: convertConfig,

		logger:  logger,
		workers: utils.NewBackgroundStoppableWorkers(),

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
	newConf, err := b.convertConfig(conf, b.logger)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.reconfigureGpios(newConf); err != nil {
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

func (b *Board) reconfigureAnalogReaders(ctx context.Context, newConf *LinuxBoardConfig) error {
	stillExists := map[string]struct{}{}
	for _, c := range newConf.AnalogReaders {
		channel, err := strconv.Atoi(c.Channel)
		if err != nil {
			return errors.Errorf("bad analog pin (%s)", c.Channel)
		}

		bus := buses.NewSpiBus(c.SPIBus)

		stillExists[c.Name] = struct{}{}
		if curr, ok := b.analogReaders[c.Name]; ok {
			if curr.chipSelect != c.ChipSelect {
				ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, c.ChipSelect}
				curr.reset(ctx, curr.chipSelect,
					pinwrappers.SmoothAnalogReader(ar, board.AnalogReaderConfig{
						AverageOverMillis: c.AverageOverMillis, SamplesPerSecond: c.SamplesPerSecond,
					}, b.logger))
			}
			continue
		}
		ar := &mcp3008helper.MCP3008AnalogReader{channel, bus, c.ChipSelect}
		b.analogReaders[c.Name] = newWrappedAnalogReader(ctx, c.ChipSelect,
			pinwrappers.SmoothAnalogReader(ar, board.AnalogReaderConfig{
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
		return &interrupt.config
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
				return err
			}
			if newGpioConfig, ok := b.gpioMappings[oldInterrupt.config.Pin]; ok {
				b.gpios[oldInterrupt.config.Pin] = b.createGpioPin(newGpioConfig)
			} else {
				b.logger.Warnf("Old interrupt pin was on nonexistent GPIO pin '%s', ignoring",
					oldInterrupt.config.Pin)
			}
		} else { // The old interrupt should stick around.
			oldInterrupt.UpdateConfig(*newConfig)
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
			// over to the new struct, if necessary.
			delete(b.interrupts, config.Name)
		}

		if oldPin, ok := b.gpios[config.Pin]; ok {
			if err := oldPin.Close(); err != nil {
				return err
			}
			delete(b.gpios, config.Pin)
		}

		// If there was an old interrupt pin with this same name, anything subscribed to the old
		// pin should still be subscribed to the new one.
		oldInterrupt, ok := oldInterrupts[config.Name]
		if !ok {
			oldInterrupt = nil
		}

		gpioMapping, ok := b.gpioMappings[config.Pin]
		if !ok {
			return fmt.Errorf("cannot create digital interrupt on unknown pin %s", config.Pin)
		}
		interrupt, err := newDigitalInterrupt(config, gpioMapping, oldInterrupt)
		if err != nil {
			return err
		}
		b.interrupts[config.Name] = interrupt
	}

	return nil
}

func (b *Board) createGpioPin(mapping GPIOBoardMapping) *gpioPin {
	startSoftwarePWMChan := make(chan any)
	pin := gpioPin{
		devicePath:           mapping.GPIOChipDev,
		offset:               uint32(mapping.GPIO),
		logger:               b.logger,
		startSoftwarePWMChan: &startSoftwarePWMChan,
	}
	pin.softwarePwm = utils.NewBackgroundStoppableWorkers(pin.softwarePwmLoop)
	if mapping.HWPWMSupported {
		pin.hwPwm = newPwmDevice(mapping.PWMSysFsDir, mapping.PWMID, b.logger)
	}
	return &pin
}

// Board implements a component for a Linux machine.
type Board struct {
	resource.Named
	mu            sync.RWMutex
	convertConfig ConfigConverter

	gpioMappings  map[string]GPIOBoardMapping
	analogReaders map[string]*wrappedAnalogReader
	logger        logging.Logger

	gpios      map[string]*gpioPin
	interrupts map[string]*digitalInterrupt

	workers *utils.StoppableWorkers
}

// AnalogByName returns the analog pin by the given name if it exists.
func (b *Board) AnalogByName(name string) (board.Analog, error) {
	a, ok := b.analogReaders[name]
	if !ok {
		return nil, errors.Errorf("can't find AnalogReader (%s)", name)
	}
	return a, nil
}

// DigitalInterruptByName returns the interrupt by the given name if it exists.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	interrupt, ok := b.interrupts[name]
	if ok {
		return interrupt, nil
	}

	// Otherwise, the name is not something we recognize yet. If it appears to be a GPIO pin, we'll
	// remove its GPIO capabilities and turn it into a digital interrupt.
	gpio, ok := b.gpios[name]
	if !ok {
		return nil, fmt.Errorf("can't find GPIO (%s)", name)
	}
	if err := gpio.Close(); err != nil {
		return nil, err
	}

	mapping, ok := b.gpioMappings[name]
	if !ok {
		return nil, fmt.Errorf("can't create digital interrupt on unknown pin %s", name)
	}
	defaultInterruptConfig := board.DigitalInterruptConfig{
		Name: name,
		Pin:  name,
	}
	interrupt, err := newDigitalInterrupt(defaultInterruptConfig, mapping, nil)
	if err != nil {
		return nil, err
	}

	delete(b.gpios, name)
	b.interrupts[name] = interrupt
	return interrupt, nil
}

// GPIOPinByName returns a GPIOPin by name.
func (b *Board) GPIOPinByName(pinName string) (board.GPIOPin, error) {
	if pin, ok := b.gpios[pinName]; ok {
		return pin, nil
	}

	// Check if pin is a digital interrupt: those can still be used as inputs.
	if interrupt, interruptOk := b.interrupts[pinName]; interruptOk {
		return interrupt, nil
	}

	return nil, errors.Errorf("cannot find GPIO for unknown pin: %s", pinName)
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

// StreamTicks starts a stream of digital interrupt ticks.
func (b *Board) StreamTicks(ctx context.Context, interrupts []board.DigitalInterrupt, ch chan board.Tick,
	extra map[string]interface{},
) error {
	var rawInterrupts []*digitalInterrupt
	for _, i := range interrupts {
		raw, ok := i.(*digitalInterrupt)
		if !ok {
			return errors.New("cannot stream ticks to an interrupt not associated with this board")
		}
		rawInterrupts = append(rawInterrupts, raw)
	}

	for _, i := range rawInterrupts {
		i.AddChannel(ch)
	}

	b.workers.Add(func(cancelCtx context.Context) {
		// Wait until it's time to shut down then remove callbacks.
		select {
		case <-ctx.Done():
		case <-cancelCtx.Done():
		}
		for _, i := range rawInterrupts {
			i.RemoveChannel(ch)
		}
	})

	return nil
}

// Close attempts to cleanly close each part of the board.
func (b *Board) Close(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.workers.Stop()

	var err error
	for _, pin := range b.gpios {
		err = multierr.Combine(err, pin.Close())
	}
	for _, interrupt := range b.interrupts {
		err = multierr.Combine(err, interrupt.Close())
	}
	for _, reader := range b.analogReaders {
		err = multierr.Combine(err, reader.Close(ctx))
	}
	return err
}
