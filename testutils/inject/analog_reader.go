package inject

import (
	"context"

	"go.viam.com/core/component/board"
)

// AnalogReader is an injected analog reader.
type AnalogReader struct {
	board.AnalogReader
	ReadFunc func(ctx context.Context) (int, error)
}

// Read calls the injected Read or the real version.
func (a *AnalogReader) Read(ctx context.Context) (int, error) {
	if a.ReadFunc == nil {
		return a.AnalogReader.Read(ctx)
	}
	return a.ReadFunc(ctx)
}
