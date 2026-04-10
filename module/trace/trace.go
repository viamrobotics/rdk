// Package trace provides a subset of the public API from
// go.viam.com/utils/trace. The API provided here is limited to what module
// authors need to instrument their code by creating new spans. Functionality
// such as changing how spans are sampled and exported is not provided, as that
// is already handled by the module framework.
package trace

import (
	"context"

	otelTrace "go.opentelemetry.io/otel/trace"
	"go.viam.com/utils/trace"
)

// StartSpan is a wrapper around [trace.Tracer.Start].
func StartSpan(ctx context.Context, name string, o ...otelTrace.SpanStartOption) (context.Context, trace.Span) {
	return trace.StartSpan(ctx, name, o...)
}

// FromContext is a wrapper around [trace.FromContext].
func FromContext(ctx context.Context) trace.Span {
	return trace.FromContext(ctx)
}

// NewContext is a wrapper around [trace.ContextWithSpan].
func NewContext(ctx context.Context, span trace.Span) context.Context {
	return trace.NewContext(ctx, span)
}
