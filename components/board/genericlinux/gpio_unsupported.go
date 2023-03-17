//go:build !linux

package genericlinux

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
)

type gpioPin struct {
	// This struct is implemented in the Linux version. We have a dummy struct here just to get
	// things to compile on non-Linux environments.
}

// We need gpioPin to implement the board.GPIOPin interface.
func (p *gpioPin) Set(ctx context.Context, high bool, extra map[string]interface{}) error {
	return errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) Get(ctx context.Context, extra map[string]interface{}) (bool, error) {
	return false, errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) PWM(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return math.NaN(), errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) SetPWM(ctx context.Context, dutyCyclePct float64, extra map[string]interface{}) error {
	return errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) PWMFreq(ctx context.Context, extra map[string]interface{}) (uint, error) {
	return 0, errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) SetPWMFreq(ctx context.Context, freqHz uint, extra map[string]interface{}) error {
	return errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

func (p *gpioPin) Close() error {
	return errors.New("GPIO pins using ioctl are not supported on non-Linux boards")
}

type digitalInterrupt struct {
	// This is another dummy struct. However, board.go uses a field within it called interrupt.
	interrupt board.DigitalInterrupt
}

func (di *digitalInterrupt) Close() error {
	return errors.New("Digital interrupts using ioctl are not supported on non-Linux boards")
}

func gpioInitialize(cancelCtx context.Context, gpioMappings map[int]GPIOBoardMapping,
	interruptConfigs []board.DigitalInterruptConfig, waitGroup *sync.WaitGroup, logger golog.Logger,
) (map[string]*gpioPin, map[string]*digitalInterrupt, error) {
	// Don't even log anything here: if someone is running in a non-Linux environment, things
	// should work fine as long as they don't try using these pins, and the log would be an
	// unnecessary warning.
	return map[string]*gpioPin{}, map[string]*digitalInterrupt{}, nil
}
