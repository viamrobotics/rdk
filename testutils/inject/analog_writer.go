package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// AnalogWriter is an injected analog writer.
type AnalogWriter struct {
	board.AnalogWriter
	WriteFunc func(ctx context.Context, value int32, extra map[string]interface{}) error
	writeCap  []interface{}
}

// Write calls the injected Write or the real version.
func (a *AnalogWriter) Write(ctx context.Context, value int32, extra map[string]interface{}) error {
	a.writeCap = []interface{}{ctx}
	if a.WriteFunc == nil {
		return a.AnalogWriter.Write(ctx, value, extra)
	}
	return a.WriteFunc(ctx, value, extra)
}

// WriteCap returns the last parameters received by Write, and then clears them.
func (a *AnalogWriter) WriteCap() []interface{} {
	if a == nil {
		return nil
	}
	defer func() { a.writeCap = nil }()
	return a.writeCap
}
