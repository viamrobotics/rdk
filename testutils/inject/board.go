package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// Board is an injected board.
type Board struct {
	board.Board
	SPIByNameFunc              func(name string) (board.SPI, bool)
	spiByNameCap               []interface{}
	I2CByNameFunc              func(name string) (board.I2C, bool)
	i2cByNameCap               []interface{}
	AnalogReaderByNameFunc     func(name string) (board.AnalogReader, bool)
	analogReaderByNameCap      []interface{}
	DigitalInterruptByNameFunc func(name string) (board.DigitalInterrupt, bool)
	digitalInterruptByNameCap  []interface{}
	SPINamesFunc               func() []string
	I2CNamesFunc               func() []string
	AnalogReaderNamesFunc      func() []string
	DigitalInterruptNamesFunc  func() []string
	CloseFunc                  func(ctx context.Context) error
	ConfigFunc                 func(ctx context.Context) (board.Config, error)
	StatusFunc                 func(ctx context.Context) (*commonpb.BoardStatus, error)
	statusCap                  []interface{}
	GPIOSetFunc                func(ctx context.Context, pin string, high bool) error
	gpioSetCap                 []interface{}
	GPIOGetFunc                func(ctx context.Context, pin string) (bool, error)
	gpioGetCap                 []interface{}
	PWMSetFunc                 func(ctx context.Context, pin string, dutyCycle byte) error
	pwmSetCap                  []interface{}
	PWMSetFreqFunc             func(ctx context.Context, pin string, freq uint) error
	pwmSetFreqCap              []interface{}
}

// SPIByName calls the injected SPIByName or the real version.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	b.spiByNameCap = []interface{}{name}
	if b.SPIByNameFunc == nil {
		return b.Board.SPIByName(name)
	}
	return b.SPIByNameFunc(name)
}

// I2CByName calls the injected I2CByName or the real version.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	b.i2cByNameCap = []interface{}{name}
	if b.I2CByNameFunc == nil {
		return b.Board.I2CByName(name)
	}
	return b.I2CByNameFunc(name)
}

// AnalogReaderByName calls the injected AnalogReaderByName or the real version.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	b.analogReaderByNameCap = []interface{}{name}
	if b.AnalogReaderByNameFunc == nil {
		return b.Board.AnalogReaderByName(name)
	}
	return b.AnalogReaderByNameFunc(name)
}

// AnalogReaderByNameCap returns the last parameters received by AnalogReaderByName, and then clears them.
func (b *Board) AnalogReaderByNameCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.analogReaderByNameCap = nil }()
	return b.analogReaderByNameCap
}

// DigitalInterruptByName calls the injected DigitalInterruptByName or the real version.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	b.digitalInterruptByNameCap = []interface{}{name}
	if b.DigitalInterruptByNameFunc == nil {
		return b.Board.DigitalInterruptByName(name)
	}
	return b.DigitalInterruptByNameFunc(name)
}

// DigitalInterruptByNameCap returns the last parameters received by DigitalInterruptByName, and then clears them.
func (b *Board) DigitalInterruptByNameCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.digitalInterruptByNameCap = nil }()
	return b.digitalInterruptByNameCap
}

// SPINames calls the injected SPINames or the real version.
func (b *Board) SPINames() []string {
	if b.SPINamesFunc == nil {
		return b.Board.SPINames()
	}
	return b.SPINamesFunc()
}

// I2CNames calls the injected SPINames or the real version.
func (b *Board) I2CNames() []string {
	if b.I2CNamesFunc == nil {
		return b.Board.I2CNames()
	}
	return b.I2CNamesFunc()
}

// AnalogReaderNames calls the injected AnalogReaderNames or the real version.
func (b *Board) AnalogReaderNames() []string {
	if b.AnalogReaderNamesFunc == nil {
		return b.Board.AnalogReaderNames()
	}
	return b.AnalogReaderNamesFunc()
}

// DigitalInterruptNames calls the injected DigitalInterruptNames or the real version.
func (b *Board) DigitalInterruptNames() []string {
	if b.DigitalInterruptNamesFunc == nil {
		return b.Board.DigitalInterruptNames()
	}
	return b.DigitalInterruptNamesFunc()
}

// Close calls the injected Close or the real version.
func (b *Board) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return utils.TryClose(ctx, b.Board)
	}
	return b.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (b *Board) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	b.statusCap = []interface{}{ctx}
	if b.StatusFunc == nil {
		return b.Board.Status(ctx)
	}
	return b.StatusFunc(ctx)
}

// StatusCap returns the last parameters received by Status, and then clears them.
func (b *Board) StatusCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.statusCap = nil }()
	return b.statusCap
}

// GPIOSet calls the injected GPIOSet or the real version.
func (b *Board) GPIOSet(ctx context.Context, pin string, high bool) error {
	b.gpioSetCap = []interface{}{ctx, pin, high}
	if b.GPIOSetFunc == nil {
		return b.Board.GPIOSet(ctx, pin, high)
	}
	return b.GPIOSetFunc(ctx, pin, high)
}

// GPIOSetCap returns the last parameters received by GPIOSet, and then clears them.
func (b *Board) GPIOSetCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.gpioSetCap = nil }()
	return b.gpioSetCap
}

// GPIOGet calls the injected GPIOGet or the real version.
func (b *Board) GPIOGet(ctx context.Context, pin string) (bool, error) {
	b.gpioGetCap = []interface{}{ctx, pin}
	if b.GPIOGetFunc == nil {
		return b.Board.GPIOGet(ctx, pin)
	}
	return b.GPIOGetFunc(ctx, pin)
}

// GPIOGetCap returns the last parameters received by GPIOGet, and then clears them.
func (b *Board) GPIOGetCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.gpioGetCap = nil }()
	return b.gpioGetCap
}

// PWMSet calls the injected PWMSet or the real version.
func (b *Board) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	b.pwmSetCap = []interface{}{ctx, pin, dutyCycle}
	if b.PWMSetFunc == nil {
		return b.Board.PWMSet(ctx, pin, dutyCycle)
	}
	return b.PWMSetFunc(ctx, pin, dutyCycle)
}

// PWMSetCap returns the last parameters received by PWMSet, and then clears them.
func (b *Board) PWMSetCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.pwmSetCap = nil }()
	return b.pwmSetCap
}

// PWMSetFreq calls the injected PWMSetFreq or the real version.
func (b *Board) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	b.pwmSetFreqCap = []interface{}{ctx, pin, freq}
	if b.PWMSetFreqFunc == nil {
		return b.Board.PWMSetFreq(ctx, pin, freq)
	}
	return b.PWMSetFreqFunc(ctx, pin, freq)
}

// PWMSetFreqCap returns the last parameters received by PWMSetFreq, and then clears them.
func (b *Board) PWMSetFreqCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.pwmSetFreqCap = nil }()
	return b.pwmSetFreqCap
}
