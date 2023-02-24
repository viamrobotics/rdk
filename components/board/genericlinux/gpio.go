//go:build linux

// Package genericlinux is for Linux boards, and this particular file is for GPIO pins using the
// ioctl interface, indirectly by way of mkch's gpio package.
package genericlinux

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/mkch/gpio"
	"go.viam.com/utils"
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
	line, err := chip.OpenLine(pin.offset, 0, gpio.Output, "viam-gpio")
	if err != nil {
		return err
	}
	pin.line = line
	return nil
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) Set(ctx context.Context, isHigh bool, extra map[string]interface{}) (err error) {
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
func (pin *gpioPin) Get(ctx context.Context, extra map[string]interface{}) (result bool, err error) {
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
	waitGroup *sync.WaitGroup, logger golog.Logger,
) map[string]*gpioPin {
	pins := make(map[string]*gpioPin)
	for pin, mapping := range gpioMappings {
		pins[fmt.Sprintf("%d", pin)] = &gpioPin{
			devicePath: mapping.GPIOChipDev,
			offset:     uint32(mapping.GPIO),
			cancelCtx:  cancelCtx,
			waitGroup:  waitGroup,
			logger:     logger,
		}
	}
	return pins
}
