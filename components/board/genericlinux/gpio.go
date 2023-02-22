//go:build linux

// Package genericlinux is for Linux boards, and this particular file is for GPIO pins using the
// ioctl interface, indirectly by way of mkch's gpio package.
package genericlinux

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/mkch/gpio"
	"github.com/pkg/errors"
	"go.viam.com/utils"
)

type gpioPin struct {
	// These values should both be considered immutable. The mutex is only here so that the use of
	// the multiple calls to the gpio package don't have race conditions.
	devicePath string
	offset     uint32
	line       *gpio.Line
	mu         sync.Mutex
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

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return math.NaN(), errors.New("PWM stuff is not supported on ioctl GPIO pins yet")
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl GPIO pins yet")
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, errors.New("PWM stuff is not supported on ioctl GPIO pins yet")
}

// This helps implement the board.GPIOPin interface for gpioPin.
func (pin *gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl GPIO pins yet")
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

func gpioInitialize(gpioMappings map[int]GPIOBoardMapping) map[string]*gpioPin {
	pins := make(map[string]*gpioPin)
	for pin, mapping := range gpioMappings {
		pins[fmt.Sprintf("%d", pin)] = &gpioPin{
			devicePath: mapping.GPIOChipDev,
			offset:     uint32(mapping.GPIO),
		}
	}
	return pins
}
