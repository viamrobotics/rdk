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
	board.Board
	name                       resource.Name
	DoFunc                     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	AnalogByNameFunc           func(name string) (board.Analog, error)
	analogByNameCap            []interface{}
	DigitalInterruptByNameFunc func(name string) (board.DigitalInterrupt, bool)
	digitalInterruptByNameCap  []interface{}
	GPIOPinByNameFunc          func(name string) (board.GPIOPin, error)
	gpioPinByNameCap           []interface{}
	AnalogNamesFunc            func() []string
	DigitalInterruptNamesFunc  func() []string
	CloseFunc                  func(ctx context.Context) error
	StatusFunc                 func(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error)
	statusCap                  []interface{}
	SetPowerModeFunc           func(ctx context.Context, mode boardpb.PowerMode, duration *time.Duration) error
	WriteAnalogFunc            func(ctx context.Context, pin string, value int32, extra map[string]interface{}) error
	StreamTicksFunc            func(ctx context.Context, interrupts []string, ch chan board.Tick, extra map[string]interface{}) error
}

// NewBoard returns a new injected board.
func NewBoard(name string) *Board {
	return &Board{name: board.Named(name)}
}

// Name returns the name of the resource.
func (b *Board) Name() resource.Name {
	return b.name
}

// AnalogByName calls the injected AnalogByName or the real version.
func (b *Board) AnalogByName(name string) (board.Analog, error) {
	b.analogByNameCap = []interface{}{name}
	if b.AnalogByNameFunc == nil {
		return b.Board.AnalogByName(name)
	}
	return b.AnalogByNameFunc(name)
}

// AnalogByNameCap returns the last parameters received by AnalogByName, and then clears them.
func (b *Board) AnalogByNameCap() []interface{} {
	if b == nil {
		return nil
	}
	defer func() { b.analogByNameCap = nil }()
	return b.analogByNameCap
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

// GPIOPinByName calls the injected GPIOPinByName or the real version.
func (b *Board) GPIOPinByName(name string) (board.GPIOPin, error) {
	b.gpioPinByNameCap = []interface{}{name}
	if b.GPIOPinByNameFunc == nil {
		return b.Board.GPIOPinByName(name)
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

// AnalogNames calls the injected AnalogNames or the real version.
func (b *Board) AnalogNames() []string {
	if b.AnalogNamesFunc == nil {
		return b.Board.AnalogNames()
	}
	return b.AnalogNamesFunc()
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
		if b.Board == nil {
			return nil
		}
		return b.Board.Close(ctx)
	}
	return b.CloseFunc(ctx)
}

// Status calls the injected Status or the real version.
func (b *Board) Status(ctx context.Context, extra map[string]interface{}) (*commonpb.BoardStatus, error) {
	b.statusCap = []interface{}{ctx}
	if b.StatusFunc == nil {
		return b.Board.Status(ctx, extra)
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
		return b.Board.DoCommand(ctx, cmd)
	}
	return b.DoFunc(ctx, cmd)
}

// SetPowerMode sets the board to the given power mode. If
// provided, the board will exit the given power mode after
// the specified duration.
func (b *Board) SetPowerMode(ctx context.Context, mode boardpb.PowerMode, duration *time.Duration) error {
	if b.SetPowerModeFunc == nil {
		return b.Board.SetPowerMode(ctx, mode, duration)
	}
	return b.SetPowerModeFunc(ctx, mode, duration)
}

// WriteAnalog calls the injected WriteAnalog or the real version.
func (b *Board) WriteAnalog(ctx context.Context, pin string, value int32, extra map[string]interface{}) error {
	if b.WriteAnalogFunc == nil {
		return b.Board.WriteAnalog(ctx, pin, value, extra)
	}
	return b.WriteAnalogFunc(ctx, pin, value, extra)
}

// StreamTicks calls the injected StreamTicks or the real version.
func (b *Board) StreamTicks(ctx context.Context, interrupts []string, ch chan board.Tick, extra map[string]interface{}) error {
	if b.StreamTicksFunc == nil {
		return b.Board.StreamTicks(ctx, interrupts, ch, extra)
	}
	return b.StreamTicksFunc(ctx, interrupts, ch, extra)
}
