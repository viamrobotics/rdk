package module

import (
	"context"
	"errors"

	v1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

type moduleOtelExporter struct {
	mod *Module
}

// Start implements otlptrace.Client.
func (m *moduleOtelExporter) Start(ctx context.Context) error {
	return nil
}

// Stop implements otlptrace.Client.
func (m *moduleOtelExporter) Stop(ctx context.Context) error {
	return nil
}

// UploadTraces implements otlptrace.Client.
func (m *moduleOtelExporter) UploadTraces(ctx context.Context, protoSpans []*v1.ResourceSpans) error {
	parent := m.mod.parent
	if parent == nil {
		return errors.New("parent connection not available")
	}
	return parent.SendTraces(ctx, protoSpans)
}
