package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// AnalogReader is an injected analog reader.
type AnalogReader struct {
	board.AnalogReader
	ReadFunc func(ctx context.Context, extra map[string]any) (int, error)
	readCap  []any
}

// Read calls the injected Read or the real version.
func (a *AnalogReader) Read(ctx context.Context, extra map[string]any) (int, error) {
	a.readCap = []any{ctx}
	if a.ReadFunc == nil {
		return a.AnalogReader.Read(ctx, extra)
	}
	return a.ReadFunc(ctx, extra)
}

// ReadCap returns the last parameters received by Read, and then clears them.
func (a *AnalogReader) ReadCap() []any {
	if a == nil {
		return nil
	}
	defer func() { a.readCap = nil }()
	return a.readCap
}
