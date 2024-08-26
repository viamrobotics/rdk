//go:build linux

// Package genericlinux is for Linux boards, and this particular file is for GPIO pins using the
// ioctl interface, indirectly by way of mkch's gpio package.
package genericlinux

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mkch/gpio"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/logging"
	rdkutils "go.viam.com/rdk/utils"
)

const noPin = 0xFFFFFFFF // noPin is the uint32 version of -1. A pin with this offset has no GPIO

type gpioPin struct {
	// These values should both be considered immutable.
	devicePath string
	offset     uint32

	// These values are mutable. Lock the mutex when interacting with them.
	line                 *gpio.Line
	isInput              bool
	hwPwm                *pwmDevice // Defined in hw_pwm.go, will be nil for pins that don't support it.
	pwmFreqHz            uint
	pwmDutyCyclePct      float64
	enableSoftwarePWM    bool      // Indicates whether a software PWM loop should continue running
	startSoftwarePWMChan *chan any // Close and reinitialize this to (re)start the SW PWM loop

	softwarePwm rdkutils.StoppableWorkers

	mu     sync.Mutex
	logger logging.Logger
}

func (pin *gpioPin) wrapError(err error) error {
	return errors.Wrapf(err, "from GPIO device %s line %d", pin.devicePath, pin.offset)
}

// This is a private helper function that should only be called when the mutex is locked. It sets
// pin.line to a valid struct or returns an error.
func (pin *gpioPin) openGpioFd(isInput bool) error {
	if isInput != pin.isInput {
		// We're switching from an input pin to an output one or vice versa. Close the line and
		// repoen in the other mode.
		if err := pin.closeGpioFd(); err != nil {
			return err // Already wrapped
		}
		pin.isInput = isInput
	}

	if pin.line != nil {
		return nil // The pin is already opened, don't re-open it.
	}

	if pin.hwPwm != nil {
		// If the pin is currently used by the hardware PWM chip, shut that down before we can open
		// it for basic GPIO use.
		if err := pin.hwPwm.Close(); err != nil {
			return pin.wrapError(err)
		}
	}

	if pin.offset == noPin {
		// This is not a GPIO pin. Now that we've turned off the PWM, return early.
		return nil
	}

	chip, err := gpio.OpenChip(pin.devicePath)
	if err != nil {
		return pin.wrapError(err)
	}
	defer utils.UncheckedErrorFunc(chip.Close)

	direction := gpio.Output
	if pin.isInput {
		direction = gpio.Input
	}

	// The 0 just means the default output value for this pin is off. We'll set it to the intended
	// value in Set(), below, if this is an output pin.
	// NOTE: we could pass in extra flags to configure the pin to be open-source or open-drain, but
	// we haven't done that yet, and we instead go with whatever the default on the board is.
	line, err := chip.OpenLine(pin.offset, 0, direction, "viam-gpio")
	if err != nil {
		return pin.wrapError(err)
	}
	pin.line = line
	return nil
}

func (pin *gpioPin) closeGpioFd() error {
	if pin.line == nil {
		return nil // The pin is already closed.
	}
	if err := pin.line.Close(); err != nil {
		return pin.wrapError(err)
	}
	pin.line = nil
	return nil
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) Set(ctx context.Context, isHigh bool,
	extra map[string]interface{},
) (err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	// Shut down any software PWM loop that might be running.
	pin.enableSoftwarePWM = false
	return pin.setInternal(isHigh)
}

// This function assumes you've already locked the mutex. It sets the value of a pin without
// changing whether the pin is part of a software PWM loop.
func (pin *gpioPin) setInternal(isHigh bool) (err error) {
	// if pin is a hw pwm pin, set to either 0% or 100% duty cycle
	if pin.hwPwm != nil {
		var value float64
		if isHigh {
			value = 1.0
		}

		// Set pwm with a frequency of 10 Hz. Any frequency value would work.
		// Note that pin.pwmFreqHz is not modified, so a previously used frequency
		// is still available to use later.
		err := pin.hwPwm.SetPwm(10, value)
		if err != nil {
			return fmt.Errorf("could not set pin: %w", err)
		}
		return nil
	}

	var value byte
	if isHigh {
		value = 1
	} else {
		value = 0
	}

	if err := pin.openGpioFd( /* isInput= */ false); err != nil {
		return err
	}

	if pin.offset == noPin {
		if isHigh {
			return errors.New("cannot set non-GPIO pin high")
		}
		// Otherwise, just return: we shut down any PWM stuff in openGpioFd.
		return nil
	}

	return pin.wrapError(pin.line.SetValue(value))
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) Get(
	ctx context.Context, extra map[string]interface{},
) (result bool, err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	if pin.hwPwm != nil {
		return false, errors.New("cannot read from a HW pwm pin")
	}

	if pin.offset == noPin {
		return false, errors.New("cannot read from non-GPIO pin")
	}

	if err := pin.openGpioFd( /* isInput= */ true); err != nil {
		return false, err
	}

	value, err := pin.line.Value()
	if err != nil {
		return false, pin.wrapError(err)
	}

	// We'd expect value to be either 0 or 1, but any non-zero value should be considered high.
	return (value != 0), nil
}

// Lock the mutex before calling this! We'll spin up a background goroutine to create a PWM signal
// in software, if we're supposed to and one isn't already running.
func (pin *gpioPin) startSoftwarePWM() error {
	if pin.pwmDutyCyclePct == 0 || pin.pwmFreqHz == 0 {
		// We don't have both parameters set up. Stop any PWM loop we might have started previously.
		pin.enableSoftwarePWM = false
		if pin.hwPwm != nil {
			return pin.hwPwm.Close()
		}
		// If we used to have a software PWM loop, we might have stopped the loop while the pin was
		// on. Remember to turn it off!
		return pin.setInternal(false)
	}

	// Otherwise, we need to output a PWM signal.
	if pin.hwPwm != nil {
		if pin.pwmFreqHz > 1 {
			if err := pin.closeGpioFd(); err != nil {
				return err
			}
			// Shut down any software PWM loop that might be running.
			pin.enableSoftwarePWM = false
			return pin.hwPwm.SetPwm(pin.pwmFreqHz, pin.pwmDutyCyclePct)
		}
		// Although this pin has hardware PWM support, many PWM chips cannot output signals at
		// frequencies this low. Stop any hardware PWM, and fall through to using a software PWM
		// loop below.
		if err := pin.hwPwm.Close(); err != nil {
			return err
		}
	}

	// If we get here, we need a software loop to drive the PWM signal, either because this pin
	// doesn't have hardware support or because we want to drive it at such a low frequency that
	// the hardware chip can't do it.
	if pin.enableSoftwarePWM {
		// We already have a software PWM loop running. It will pick up the changes on its own.
		return nil
	}

	pin.enableSoftwarePWM = true
	// Sneaky trick alert! We use startSoftwarePWMChan to tell the background worker to start
	// toggling the pin in a loop. However, we have locked the mutex, and if the background worker
	// is already in progress (because the software PWM loop had been started, another goroutine
	// told it to pause, and we are running so soon afterwards that it hasn't yet noticed it should
	// pause), it's possible the background worker is still looping and is about to lock the mutex
	// so it can toggle the pin. If we tried sending a message on the channel in that situation, we
	// would deadlock (we would wait until something can read from the channel, and the only
	// goroutine that can read from it would wait until the mutex is unlocked). So, instead, we
	// send the message by closing the old channel and creating a new one: if the background worker
	// was waiting on the channel, it will wake up when it closes, and if it was not waiting on the
	// channel, we don't block waiting for it to check the channel.
	newSoftwarePWMChan := make(chan any)
	close(*pin.startSoftwarePWMChan)
	pin.startSoftwarePWMChan = &newSoftwarePWMChan
	return nil
}

// accurateSleep is intended to be a replacement for utils.SelectContextOrWait which wakes up
// closer to when it's supposed to. We return whether the context is still valid (not yet
// cancelled).
func accurateSleep(ctx context.Context, duration time.Duration) bool {
	// If we use utils.SelectContextOrWait(), we will wake up sometime after when we're supposed
	// to, which can be hundreds of microseconds later (because the process scheduler in the OS only
	// schedules things every millisecond or two). For use cases like a web server responding to a
	// query, that's fine. but when outputting a PWM signal, hundreds of microseconds can be a big
	// deal. To avoid this, we sleep for less time than we're supposed to, and then busy-wait until
	// the right time. Inspiration for this approach was taken from
	// https://blog.bearcats.nl/accurate-sleep-function/
	// On a raspberry pi 4, naively calling utils.SelectContextOrWait tended to have an error of
	// about 140-300 microseconds, while this version had an error of 0.3-0.6 microseconds.
	startTime := time.Now()
	maxBusyWaitTime := 1500 * time.Microsecond
	if duration > maxBusyWaitTime {
		shorterDuration := duration - maxBusyWaitTime
		if !utils.SelectContextOrWait(ctx, shorterDuration) {
			return false
		}
	}

	for time.Since(startTime) < duration {
		if err := ctx.Err(); err != nil {
			return false
		}
		// Otherwise, busy-wait some more
	}
	return true
}

// We turn the pin either on or off, and then wait until it's time to turn it off or on again (or
// until we're supposed to shut down). We return whether we should continue the software PWM cycle.
func (pin *gpioPin) halfPwmCycle(ctx context.Context, shouldBeOn bool) bool {
	// Make local copies of these, then release the mutex
	var dutyCycle float64
	var freqHz uint

	// We encapsulate some of this code into its own function, to ensure that the mutex is unlocked
	// at the appropriate time even if we return early.
	shouldContinue := func() bool {
		pin.mu.Lock()
		defer pin.mu.Unlock()
		// Before we modify the pin, check if we should stop running
		if ctx.Err() != nil || !pin.enableSoftwarePWM {
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

	return accurateSleep(ctx, duration)
}

func (pin *gpioPin) softwarePwmLoop(ctx context.Context) {
	for {
		// It's possible another goroutine has closed startSoftwarePWMCHan and is reinitializing
		// it, and we might be running on a board with a small enough CPU that pointer assignment
		// is not atomic. Lock the mutex when getting a copy of the channel, so we don't
		// accidentally get half a pointer to an old one and half a pointer to a new one.
		startSoftwarePWMChan := func() chan any {
			pin.mu.Lock()
			defer pin.mu.Unlock()
			return *pin.startSoftwarePWMChan
		}()

		select {
		case <-ctx.Done():
			return
		case <-startSoftwarePWMChan:
		}
		for {
			if !pin.halfPwmCycle(ctx, true) {
				break
			}
			if !pin.halfPwmCycle(ctx, false) {
				break
			}
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

	dutyCyclePct, err := board.ValidatePWMDutyCycle(dutyCyclePct)
	if err != nil {
		return err
	}

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

	if freqHz < 1 {
		return errors.New("must set PWM frequency to a positive value")
	}

	pin.pwmFreqHz = freqHz
	return pin.startSoftwarePWM()
}

func (pin *gpioPin) Close() error {
	// We keep the gpio.Line object open indefinitely, so it holds its state for as long as this
	// struct is around. This function is a way to close it when we're about to go out of scope, so
	// we don't leak file descriptors.

	// WARNING: the background worker locks the mutex when it interacts with our mutable state, so
	// we need to stop it when the mutex is unlocked, or risk deadlock!
	pin.softwarePwm.Stop()
	pin.mu.Lock()
	defer pin.mu.Unlock()

	if pin.hwPwm != nil {
		if err := pin.hwPwm.Close(); err != nil {
			return err
		}
	}

	// If a pin has never been used, leave it alone. This is more important than you might expect:
	// on some boards (e.g., the Beaglebone AI-64), turning off a GPIO pin tells the kernel that
	// the pin is in use by the GPIO system and therefore it cannot be used for I2C or other
	// functions. Make sure that closing a pin here doesn't disable I2C!
	if pin.line != nil {
		// If the entire server is shutting down, it's important to turn off all pins so they don't
		// continue outputting signals we can no longer control.
		if err := pin.setInternal(false); err != nil { // setInternal won't double-lock the mutex
			return err
		}

		if err := pin.closeGpioFd(); err != nil {
			return err
		}
	}

	return nil
}
