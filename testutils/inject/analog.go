package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// Analog is an injected analog pin.
type Analog struct {
	board.Analog
	ReadFunc func(ctx context.Context, extra map[string]interface{}) (int, error)
	readCap  []interface{}
}

// Read calls the injected Read or the real version.
func (a *Analog) Read(ctx context.Context, extra map[string]interface{}) (int, error) {
	a.readCap = []interface{}{ctx}
	if a.ReadFunc == nil {
		return a.Analog.Read(ctx, extra)
	}
	return a.ReadFunc(ctx, extra)
}

// ReadCap returns the last parameters received by Read, and then clears them.
func (a *Analog) ReadCap() []interface{} {
	if a == nil {
		return nil
	}
	defer func() { a.readCap = nil }()
	return a.readCap
}
