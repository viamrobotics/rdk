//go:build linux

// Package genericlinux is for Linux boards, and this particular file is for GPIO pins using the
// ioctl interface, indirectly by way of mkch's gpio package.
package genericlinux

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mkch/gpio"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
)

type gpioPin struct {
	// These values should both be considered immutable.
	devicePath string
	offset     uint32
	line       *gpio.Line

	// These values are mutable. Lock the mutex when interacting with them.
	pwmRunning      bool
	pwmFreqHz       uint
	pwmDutyCyclePct float64

	mu        sync.Mutex
	cancelCtx context.Context
	waitGroup *sync.WaitGroup
	logger    golog.Logger
}

// This is a private helper function that should only be called when the mutex is locked. It sets
// pin.line to a valid struct or returns an error.
func (pin *gpioPin) openGpioFd() error {
	if pin.line != nil {
		return nil // If the pin is already opened, don't re-open it.
	}

	chip, err := gpio.OpenChip(pin.devicePath)
	if err != nil {
		return err
	}
	defer utils.UncheckedErrorFunc(chip.Close)

	// The 0 just means the default value for this pin is off. We'll set it to the intended value
	// in Set(), below.
	// NOTE: we could pass in extra flags to configure the pin to be open-source or open-drain, but
	// we haven't done that yet, and instead go with whatever the default on the board is.
	line, err := chip.OpenLine(pin.offset, 0, gpio.Output, "viam-gpio")
	if err != nil {
		return err
	}
	pin.line = line
	return nil
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) Set(ctx context.Context, isHigh bool,
	extra map[string]interface{},
) (err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	if err := pin.openGpioFd(); err != nil {
		return err
	}

	pin.pwmRunning = false
	return pin.setInternal(isHigh)
}

// This function assumes you've already locked the mutex. It sets the value of a pin without
// changing whether the pin is part of a PWM loop.
func (pin *gpioPin) setInternal(isHigh bool) (err error) {
	var value byte
	if isHigh {
		value = 1
	} else {
		value = 0
	}

	return pin.line.SetValue(value)
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) Get(
	ctx context.Context, extra map[string]interface{},
) (result bool, err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	if err := pin.openGpioFd(); err != nil {
		return false, err
	}

	value, err := pin.line.Value()
	if err != nil {
		return false, err
	}

	// We'd expect value to be either 0 or 1, but any non-zero value should be considered high.
	return (value != 0), nil
}

// Lock the mutex before calling this! We'll spin up a background goroutine to create a PWM signal
// in software, if we're supposed to and one isn't already running.
func (pin *gpioPin) startSoftwarePWM() error {
	if pin.pwmDutyCyclePct == 0 || pin.pwmFreqHz == 0 {
		// We don't have both parameters set up. Stop any PWM loop we might have started already.
		pin.pwmRunning = false
		// If we used to have both parameters set but no longer do, turn off the pin!
		return pin.setInternal(false)
	}
	if pin.pwmRunning {
		// We're already running a software PWM loop for this pin, so we don't need another.
		return nil
	}

	// Otherwise, we'll actually start running.
	pin.pwmRunning = true
	pin.waitGroup.Add(1)
	utils.ManagedGo(pin.softwarePwmLoop, pin.waitGroup.Done)
	return nil
}

// We turn the pin either on or off, and then wait until it's time to turn it off or on again (or
// until we're supposed to shut down). We return whether we should continue the software PWM cycle.
func (pin *gpioPin) halfPwmCycle(shouldBeOn bool) bool {
	// Make local copies of these, then release the mutex
	var dutyCycle float64
	var freqHz uint

	// We encapsulate some of this code into its own function, to ensure that the mutex is unlocked
	// at the appropriate time even if we return early.
	shouldContinue := func() bool {
		pin.mu.Lock()
		defer pin.mu.Unlock()
		// Before we modify the pin, check if we should stop running
		if !pin.pwmRunning {
			return false
		}

		dutyCycle = pin.pwmDutyCyclePct
		freqHz = pin.pwmFreqHz

		// If there's an error turning the pin on or off, don't stop the whole loop. Hopefully we
		// can toggle it next time. However, log any errors so that we notice if there are a bunch
		// of them.
		utils.UncheckedErrorFunc(func() error { return pin.setInternal(shouldBeOn) })
		return true
	}()

	if !shouldContinue {
		return false
	}

	if !shouldBeOn {
		dutyCycle = 1 - dutyCycle
	}
	duration := time.Duration(float64(time.Second) * dutyCycle / float64(freqHz))
	return utils.SelectContextOrWait(pin.cancelCtx, duration)
}

func (pin *gpioPin) softwarePwmLoop() {
	for {
		if !pin.halfPwmCycle(true) {
			return
		}
		if !pin.halfPwmCycle(false) {
			return
		}
	}
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	return pin.pwmDutyCyclePct, nil
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	pin.pwmDutyCyclePct = dutyCyclePct
	return pin.startSoftwarePWM()
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	return pin.pwmFreqHz, nil
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	pin.pwmFreqHz = freqHz
	return pin.startSoftwarePWM()
}

func (pin *gpioPin) Close() error {
	// We keep the gpio.Line object open indefinitely, so it holds its state for as long as this
	// struct is around. This function is a way to close it when we're about to go out of scope, so
	// we don't leak file descriptors.
	pin.mu.Lock()
	defer pin.mu.Unlock()

	if pin.line == nil {
		return nil // Never opened, so no need to close
	}

	err := pin.line.Close()
	pin.line = nil
	return err
}

func gpioInitialize(cancelCtx context.Context, gpioMappings map[int]GPIOBoardMapping,
	interruptConfigs []board.DigitalInterruptConfig, waitGroup *sync.WaitGroup, logger golog.Logger,
) (map[string]*gpioPin, map[string]*digitalInterrupt, error) {
	interrupts := make(map[string]*digitalInterrupt, len(interruptConfigs))
	for _, config := range interruptConfigs {
		interrupt, err := createDigitalInterrupt(cancelCtx, config, gpioMappings, waitGroup)
		if err != nil {
			// Close all pins we've started
			for _, runningInterrupt := range interrupts {
				err = multierr.Combine(err, runningInterrupt.Close())
			}
			return nil, nil, err
		}
		interrupts[config.Pin] = interrupt
	}

	pins := make(map[string]*gpioPin)
	for pin, mapping := range gpioMappings {
		if _, ok := interrupts[fmt.Sprintf("%d", pin)]; ok {
			logger.Debugf(
				"Skipping initialization of GPIO pin %s because it's configured as an interrupt",
				pin)
			continue
		}
		pins[fmt.Sprintf("%d", pin)] = &gpioPin{
			devicePath: mapping.GPIOChipDev,
			offset:     uint32(mapping.GPIO),
			cancelCtx:  cancelCtx,
			waitGroup:  waitGroup,
			logger:     logger,
		}
	}
	return pins, interrupts, nil
}

type digitalInterrupt struct {
	interrupt  board.DigitalInterrupt
	line       *gpio.LineWithEvent
	cancelCtx  context.Context
	cancelFunc func()
}

func createDigitalInterrupt(ctx context.Context, config board.DigitalInterruptConfig,
	gpioMappings map[int]GPIOBoardMapping, activeBackgroundWorkers *sync.WaitGroup,
) (*digitalInterrupt, error) {
	pinInt, err := strconv.Atoi(config.Pin)
	if err != nil {
		return nil, errors.Errorf("pin names must be numerical, not '%s'", config.Pin)
	}
	mapping, ok := gpioMappings[pinInt]
	if !ok {
		return nil, errors.Errorf("Unknown interrupt pin %s", config.Pin)
	}

	chip, err := gpio.OpenChip(mapping.GPIOChipDev)
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(chip.Close)

	line, err := chip.OpenLineWithEvents(
		uint32(mapping.GPIO), gpio.Input, gpio.BothEdges, "viam-interrupt")
	if err != nil {
		return nil, err
	}

	interrupt, err := board.CreateDigitalInterrupt(config)
	if err != nil {
		return nil, multierr.Combine(err, line.Close())
	}

	cancelCtx, cancelFunc := context.WithCancel(ctx)
	result := digitalInterrupt{
		interrupt:  interrupt,
		line:       line,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	result.startMonitor(activeBackgroundWorkers)
	return &result, nil
}

func (di *digitalInterrupt) startMonitor(activeBackgroundWorkers *sync.WaitGroup) {
	activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		for {
			select {
			case <-di.cancelCtx.Done():
				return
			case event := <-di.line.Events():
				utils.UncheckedError(di.interrupt.Tick(
					di.cancelCtx, event.RisingEdge, uint64(event.Time.UnixNano())))
			}
		}
	}, activeBackgroundWorkers.Done)
}

func (di *digitalInterrupt) Close() error {
	// We shut down the background goroutine that monitors this interrupt, but don't need to wait
	// for it to finish shutting down because it doesn't use anything in the line itself (just a
	// channel of events that the line generates). It will shut down sometime soon, and if that's
	// after the line is closed, that's fine.
	di.cancelFunc()
	return di.line.Close()
}
