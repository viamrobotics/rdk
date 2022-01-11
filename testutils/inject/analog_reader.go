package inject

import (
	"context"

	"go.viam.com/rdk/component/board"
)

// AnalogReader is an injected analog reader.
type AnalogReader struct {
	board.AnalogReader
	ReadFunc func(ctx context.Context) (int, error)
	readCap  []interface{}
}

// Read calls the injected Read or the real version.
func (a *AnalogReader) Read(ctx context.Context) (int, error) {
	a.readCap = []interface{}{ctx}
	if a.ReadFunc == nil {
		return a.AnalogReader.Read(ctx)
	}
	return a.ReadFunc(ctx)
}

// ReadCap returns the last parameters received by Read, and then clears them.
func (a *AnalogReader) ReadCap() []interface{} {
	if a == nil {
		return nil
	}
	defer func() { a.readCap = nil }()
	return a.readCap
}
