package inject

import (
	"context"

	"go.viam.com/rdk/components/board"
)

// AnalogWriter is an injected analog writer.
type AnalogWriter struct {
	board.AnalogWriter
	WriteFunc    func(ctx context.Context, value int32, extra map[string]interface{}) error
	writeCapture []interface{}
}

// Write calls the injected Write or the real version.
func (a *AnalogWriter) Write(ctx context.Context, value int32, extra map[string]interface{}) error {
	a.writeCapture = []interface{}{ctx, value}
	if a.WriteFunc == nil {
		return a.AnalogWriter.Write(ctx, value, extra)
	}
	return a.WriteFunc(ctx, value, extra)
}

// WriteCapture returns the last parameters received by Write, and then clears them.
func (a *AnalogWriter) WriteCapture() []interface{} {
	if a == nil {
		return nil
	}
	defer func() { a.writeCapture = nil }()
	return a.writeCapture
}
