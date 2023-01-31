//go:build linux
package genericlinux

// This file is heavily inspired by https://github.com/mkch/gpio

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/mkch/gpio"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
)

type ioctlPin struct {
	// These values should both be considered immutable. The mutex is only here so that the use of
	// the multiple calls to the gpio package don't have race conditions.
	devicePath string
	offset     uint32
	mu         sync.Mutex
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) Set(ctx context.Context, isHigh bool, extra map[string]interface{}) (err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	var value byte
	if isHigh {
		value = 1
	} else {
		value = 0
	}

	chip, err := gpio.OpenChip(pin.devicePath)
	if err != nil {
		return err
	}
	defer func() {err = chip.Close()}()

	line, err := chip.OpenLine(pin.offset, value, gpio.Output, "viam-gpio")
	if err != nil {
		return err
	}
	defer func() {err = line.Close()}()

	return nil
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) Get(ctx context.Context, extra map[string]interface{}) (bool, err error) {
	pin.mu.Lock()
	defer pin.mu.Unlock()

	chip, err := gpio.OpenChip(pin.devicePath)
	if err != nil {
		return false, err
	}
	defer func() {err = chip.Close()}()

	line, err := chip.OpenLine(pin.offset, 0, gpio.Input, "viam-gpio")
	if err != nil {
		return false, err
	}
	defer func() {err = line.Close()}()

	value, err := line.Value()
	if err != nil {
		return false, err
	}

	// We'd expect value to be either 0 or 1, but any non-zero value should be considered high.
	return (value != 0), nil
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return math.NaN(), errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, errors.New("PWM stuff is not supported on ioctl pins yet")
}

// This helps implement the board.GPIOPin interface for ioctlPin.
func (pin *ioctlPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return errors.New("PWM stuff is not supported on ioctl pins yet")
}

var pins map[string]*ioctlPin

func ioctlInitialize(gpioMappings map[int]GPIOBoardMapping) {
	pins = make(map[string]*ioctlPin)
	for pin, mapping := range gpioMappings {
		pins[fmt.Sprintf("%d", pin)] = &ioctlPin{
			devicePath: mapping.GPIOChipDev,
			offset:     uint32(mapping.GPIO),
		}
	}
}

func ioctlGetPin(pinName string) (board.GPIOPin, error) {
	pin, ok := pins[pinName]
	if !ok {
		return nil, errors.Errorf("Cannot set GPIO for unknown pin: %s", pinName)
	}
	return pin, nil
}
