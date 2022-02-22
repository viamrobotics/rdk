package inject

import (
	"context"

	"go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
)

// Board is an injected board.
type Board struct {
	board.LocalBoard
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
	SetGPIOFunc                func(ctx context.Context, pin string, high bool) error
	setGPIOCap                 []interface{}
	GetGPIOFunc                func(ctx context.Context, pin string) (bool, error)
	getGPIOCap                 []interface{}
	SetPWMFunc                 func(ctx context.Context, pin string, dutyCyclePct float64) error
	setPWMCap                  []interface{}
	SetPWMFreqFunc             func(ctx context.Context, pin string, freqHz uint) error
	setPWMFreqCap              []interface{}
}

// SPIByName calls the injected SPIByName or the real version.
func (b *Board) SPIByName(name string) (board.SPI, bool) {
	b.spiByNameCap = []interface{}{name}
	if b.SPIByNameFunc == nil {
		return b.LocalBoard.SPIByName(name)
	}
	return b.SPIByNameFunc(name)
}

// I2CByName calls the injected I2CByName or the real version.
func (b *Board) I2CByName(name string) (board.I2C, bool) {
	b.i2cByNameCap = []interface{}{name}
	if b.I2CByNameFunc == nil {
		return b.LocalBoard.I2CByName(name)
	}
	return b.I2CByNameFunc(name)
}

// AnalogReaderByName calls the injected AnalogReaderByName or the real version.
func (b *Board) AnalogReaderByName(name string) (board.AnalogReader, bool) {
	b.analogReaderByNameCap = []interface{}{name}
	if b.AnalogReaderByNameFunc == nil {
		return b.LocalBoard.AnalogReaderByName(name)
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
		return b.LocalBoard.DigitalInterruptByName(name)
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
		return b.LocalBoard.SPINames()
	}
	return b.SPINamesFunc()
}

// I2CNames calls the injected SPINames or the real version.
func (b *Board) I2CNames() []string {
	if b.I2CNamesFunc == nil {
		return b.LocalBoard.I2CNames()
	}
	return b.I2CNamesFunc()
}

// AnalogReaderNames calls the injected AnalogReaderNames or the real version.
func (b *Board) AnalogReaderNames() []string {
	if b.AnalogReaderNamesFunc == nil {
		return b.LocalBoard.AnalogReaderNames()
	}
	return b.AnalogReaderNamesFunc()
}

// DigitalInterruptNames calls the injected DigitalInterruptNames or the real version.
func (b *Board) DigitalInterruptNames() []string {
	if b.DigitalInterruptNamesFunc == nil {
		return b.LocalBoard.DigitalInterruptNames()
	}
	return b.DigitalInterruptNamesFunc()
}

// Close calls the injected Close or the real version.
func (b *Board) Close(ctx context.Context) error {
	if b.CloseFunc == nil {
		return utils.TryClose(ctx, b.LocalBoard)
	}
	return b.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (b *Board) Status(ctx context.Context) (*commonpb.BoardStatus, error) {
	b.statusCap = []interface{}{ctx}
	if b.StatusFunc == nil {
		return b.LocalBoard.Status(ctx)
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

// SetGPIO calls the injected SetGPIO or the real version.
func (b *Board) SetGPIO(ctx context.Context, pin string, high bool) error {
	b.setGPIOCap = []interface{}{ctx, pin, high}
	if b.SetGPIOFunc == nil {
		return b.LocalBoard.SetGPIO(ctx, pin, high)
	}
	return b.SetGPIOFunc(ctx, pin, high)
}

// SetGPIOCap returns the last parameters received by SetGPIO, and then clears them.
func (b *Board) SetGPIOCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.setGPIOCap = nil }()
	return b.setGPIOCap
}

// GetGPIO calls the injected GetGPIO or the real version.
func (b *Board) GetGPIO(ctx context.Context, pin string) (bool, error) {
	b.getGPIOCap = []interface{}{ctx, pin}
	if b.GetGPIOFunc == nil {
		return b.LocalBoard.GetGPIO(ctx, pin)
	}
	return b.GetGPIOFunc(ctx, pin)
}

// GetGPIOCap returns the last parameters received by GetGPIO, and then clears them.
func (b *Board) GetGPIOCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.getGPIOCap = nil }()
	return b.getGPIOCap
}

// SetPWM calls the injected SetPWM or the real version.
func (b *Board) SetPWM(ctx context.Context, pin string, dutyCyclePct float64) error {
	b.setPWMCap = []interface{}{ctx, pin, dutyCyclePct}
	if b.SetPWMFunc == nil {
		return b.LocalBoard.SetPWM(ctx, pin, dutyCyclePct)
	}
	return b.SetPWMFunc(ctx, pin, dutyCyclePct)
}

// SetPWMCap returns the last parameters received by SetPWM, and then clears them.
func (b *Board) SetPWMCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.setPWMCap = nil }()
	return b.setPWMCap
}

// SetPWMFreq calls the injected SetPWMFreq or the real version.
func (b *Board) SetPWMFreq(ctx context.Context, pin string, freqHz uint) error {
	b.setPWMFreqCap = []interface{}{ctx, pin, freqHz}
	if b.SetPWMFreqFunc == nil {
		return b.LocalBoard.SetPWMFreq(ctx, pin, freqHz)
	}
	return b.SetPWMFreqFunc(ctx, pin, freqHz)
}

// SetPWMFreqCap returns the last parameters received by SetPWMFreq, and then clears them.
func (b *Board) SetPWMFreqCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.setPWMFreqCap = nil }()
	return b.setPWMFreqCap
}
