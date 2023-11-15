package inject

import (
	"context"
	"time"

	commonpb "go.viam.com/api/common/v1"
	boardpb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/resource"
)

// Board is an injected board.
type Board struct {
	board.LocalBoard
	name                       resource.Name
	DoFunc                     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	I2CByNameFunc              func(name string) (board.I2C, bool)
	i2cByNameCap               []interface{}
	AnalogReaderByNameFunc     func(name string) (board.AnalogReader, bool)
	analogReaderByNameCap      []interface{}
	DigitalInterruptByNameFunc func(name string) (board.DigitalInterrupt, bool)
	digitalInterruptByNameCap  []interface{}
	GPIOPinByNameFunc          func(name string) (board.GPIOPin, error)
	gpioPinByNameCap           []interface{}
	AnalogReaderNamesFunc      func() []string
	DigitalInterruptNamesFunc  func() []string
	CloseFunc                  func(ctx context.Context) error
	StatusFunc                 func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error)
	statusCap                  []interface{}
	SetPowerModeFunc           func(ctx context.Context, mode boardpb.PowerMode, duration *time.Duration) error
	WriteAnalogFunc            func(ctx context.Context, pin string, value int32, extra map[string]interface{}) error
}

// NewBoard returns a new injected board.
func NewBoard(name string) *Board {
	return &Board{name: board.Named(name)}
}

// Name returns the name of the resource.
func (b *Board) Name() resource.Name {
	return b.name
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

// GPIOPinByName calls the injected GPIOPinByName or the real version.
func (b *Board) GPIOPinByName(name string) (board.GPIOPin, error) {
	b.gpioPinByNameCap = []interface{}{name}
	if b.GPIOPinByNameFunc == nil {
		return b.LocalBoard.GPIOPinByName(name)
	}
	return b.GPIOPinByNameFunc(name)
}

// GPIOPinByNameCap returns the last parameters received by GPIOPinByName, and then clears them.
func (b *Board) GPIOPinByNameCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.gpioPinByNameCap = nil }()
	return b.gpioPinByNameCap
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
		if b.LocalBoard == nil {
			return nil
		}
		return b.LocalBoard.Close(ctx)
	}
	return b.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (b *Board) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	b.statusCap = []interface{}{ctx}
	if b.StatusFunc == nil {
		return b.LocalBoard.Status(ctx, extra)
	}
	return b.StatusFunc(ctx, extra)
}

// StatusCap returns the last parameters received by Status, and then clears them.
func (b *Board) StatusCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.statusCap = nil }()
	return b.statusCap
}

// DoCommand calls the injected DoCommand or the real version.
func (b *Board) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if b.DoFunc == nil {
		return b.LocalBoard.DoCommand(ctx, cmd)
	}
	return b.DoFunc(ctx, cmd)
}

// SetPowerMode sets the board to the given power mode. If
// provided, the board will exit the given power mode after
// the specified duration.
func (b *Board) SetPowerMode(ctx context.Context, mode boardpb.PowerMode, duration *time.Duration) error {
	if b.SetPowerModeFunc == nil {
		return b.LocalBoard.SetPowerMode(ctx, mode, duration)
	}
	return b.SetPowerModeFunc(ctx, mode, duration)
}

// WriteAnalog calls the injected WriteAnalog or the real version.
func (b *Board) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	if b.WriteAnalogFunc == nil {
		return b.LocalBoard.WriteAnalog(ctx, pin, value, extra)
	}
	return b.WriteAnalogFunc(ctx, pin, value, extra)
}
