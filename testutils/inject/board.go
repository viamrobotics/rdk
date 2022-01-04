package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	pb "go.viam.com/rdk/proto/api/v1"
)

// Board is an injected board.
type Board struct {
	board.Board
	SPIByNameFunc              func(name string) (board.SPI, bool)
	SPIByNameCap               []interface{}
	I2CByNameFunc              func(name string) (board.I2C, bool)
	I2CByNameCap               []interface{}
	AnalogReaderByNameFunc     func(name string) (board.AnalogReader, bool)
	AnalogReaderByNameCap      []interface{}
	DigitalInterruptByNameFunc func(name string) (board.DigitalInterrupt, bool)
	DigitalInterruptByNameCap  []interface{}
	SPINamesFunc               func() []string
	SPINamesCap                []interface{}
	I2CNamesFunc               func() []string
	I2CNamesCap                []interface{}
	AnalogReaderNamesFunc      func() []string
	AnalogReaderNamesCap       []interface{}
	DigitalInterruptNamesFunc  func() []string
	DigitalInterruptNamesCap   []interface{}
	CloseFunc                  func(ctx context.Context) error
	CloseCap                   []interface{}
	ConfigFunc                 func(ctx context.Context) (board.Config, error)
	ConfigCap                  []interface{}
	StatusFunc                 func(ctx context.Context) (*pb.BoardStatus, error)
	StatusCap                  []interface{}
	GPIOSetFunc                func(ctx context.Context, pin string, high bool) error
	GPIOSetCap                 []interface{}
	GPIOGetFunc                func(ctx context.Context, pin string) (bool, error)
	GPIOGetCap                 []interface{}
	PWMSetFunc                 func(ctx context.Context, pin string, dutyCycle byte) error
	PWMSetCap                  []interface{}
	PWMSetFreqFunc             func(ctx context.Context, pin string, freq uint) error
	PWMSetFreqCap              []interface{}
}

// SPIByName calls the injected SPIByName or the real version.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	b.SPIByNameCap = []interface{}{name}
	if b.SPIByNameFunc == nil {
		return b.Board.SPIByName(name)
	}
	return b.SPIByNameFunc(name)
}

// I2CByName calls the injected I2CByName or the real version.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	b.I2CByNameCap = []interface{}{name}
	if b.I2CByNameFunc == nil {
		return b.Board.I2CByName(name)
	}
	return b.I2CByNameFunc(name)
}

// AnalogReaderByName calls the injected AnalogReaderByName or the real version.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	b.AnalogReaderByNameCap = []interface{}{name}
	if b.AnalogReaderByNameFunc == nil {
		return b.Board.AnalogReaderByName(name)
	}
	return b.AnalogReaderByNameFunc(name)
}

// DigitalInterruptByName calls the injected DigitalInterruptByName or the real version.
func (b *Board) DigitalInterruptByName(name string) (board.DigitalInterrupt, bool) {
	b.DigitalInterruptByNameCap = []interface{}{name}
	if b.DigitalInterruptByNameFunc == nil {
		return b.Board.DigitalInterruptByName(name)
	}
	return b.DigitalInterruptByNameFunc(name)
}

// SPINames calls the injected SPINames or the real version.
func (b *Board) SPINames() []string {
	b.SPINamesCap = []interface{}{}
	if b.SPINamesFunc == nil {
		return b.Board.SPINames()
	}
	return b.SPINamesFunc()
}

// I2CNames calls the injected SPINames or the real version.
func (b *Board) I2CNames() []string {
	b.I2CNamesCap = []interface{}{}
	if b.I2CNamesFunc == nil {
		return b.Board.I2CNames()
	}
	return b.I2CNamesFunc()
}

// AnalogReaderNames calls the injected AnalogReaderNames or the real version.
func (b *Board) AnalogReaderNames() []string {
	b.AnalogReaderNamesCap = []interface{}{}
	if b.AnalogReaderNamesFunc == nil {
		return b.Board.AnalogReaderNames()
	}
	return b.AnalogReaderNamesFunc()
}

// DigitalInterruptNames calls the injected DigitalInterruptNames or the real version.
func (b *Board) DigitalInterruptNames() []string {
	b.DigitalInterruptNamesCap = []interface{}{}
	if b.DigitalInterruptNamesFunc == nil {
		return b.Board.DigitalInterruptNames()
	}
	return b.DigitalInterruptNamesFunc()
}

// Close calls the injected Close or the real version.
func (b *Board) Close(ctx context.Context) error {
	b.CloseCap = []interface{}{ctx}
	if b.CloseFunc == nil {
		return utils.TryClose(ctx, b.Board)
	}
	return b.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (b *Board) Status(ctx context.Context) (*pb.BoardStatus, error) {
	b.StatusCap = []interface{}{ctx}
	if b.StatusFunc == nil {
		return b.Board.Status(ctx)
	}
	return b.StatusFunc(ctx)
}

// GPIOSet calls the injected GPIOSet or the real version.
func (b *Board) GPIOSet(ctx context.Context, pin string, high bool) error {
	b.GPIOSetCap = []interface{}{ctx, pin, high}
	if b.GPIOSetFunc == nil {
		return b.Board.GPIOSet(ctx, pin, high)
	}
	return b.GPIOSetFunc(ctx, pin, high)
}

// GPIOGet calls the injected GPIOGet or the real version.
func (b *Board) GPIOGet(ctx context.Context, pin string) (bool, error) {
	b.GPIOGetCap = []interface{}{ctx, pin}
	if b.GPIOGetFunc == nil {
		return b.Board.GPIOGet(ctx, pin)
	}
	return b.GPIOGetFunc(ctx, pin)
}

// PWMSet calls the injected PWMSet or the real version.
func (b *Board) PWMSet(ctx context.Context, pin string, dutyCycle byte) error {
	b.PWMSetCap = []interface{}{ctx, pin, dutyCycle}
	if b.PWMSetFunc == nil {
		return b.Board.PWMSet(ctx, pin, dutyCycle)
	}
	return b.PWMSetFunc(ctx, pin, dutyCycle)
}

// PWMSetFreq calls the injected PWMSetFreq or the real version.
func (b *Board) PWMSetFreq(ctx context.Context, pin string, freq uint) error {
	b.PWMSetFreqCap = []interface{}{ctx, pin, freq}
	if b.PWMSetFreqFunc == nil {
		return b.Board.PWMSetFreq(ctx, pin, freq)
	}
	return b.PWMSetFreqFunc(ctx, pin, freq)
}
