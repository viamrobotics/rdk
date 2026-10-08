// Package otelmetrics installs the process-wide OpenTelemetry MeterProvider for viam-server.
package otelmetrics

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.uber.org/multierr"
)

// Enabled reports whether an OTLP endpoint is set through the standard OTEL_ environment variables.
func Enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// Start sets a global MeterProvider that exports over OTLP/gRPC as the OTEL_ environment variables
// configure it, and starts Go runtime and host metrics. The returned function flushes and stops it.
func Start(ctx context.Context, partID, version string) (func(context.Context) error, error) {
	res, err := newResource(ctx, partID, version)
	if err != nil {
		return nil, err
	}
	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		return nil, multierr.Combine(err, provider.Shutdown(ctx))
	}
	if err := host.Start(host.WithMeterProvider(provider)); err != nil {
		return nil, multierr.Combine(err, provider.Shutdown(ctx))
	}
	otel.SetMeterProvider(provider)
	return provider.Shutdown, nil
}

// newResource leaves the viam attributes schemaless so they merge with whatever semconv schema
// the SDK's detectors use; two different schema URLs fail the merge.
func newResource(ctx context.Context, partID, version string) (*otelresource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName("rdk"), semconv.ServiceNamespace("viam.com")}
	if version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}
	if partID != "" {
		attrs = append(attrs, attribute.String("viam.part.id", partID))
	}
	return otelresource.New(ctx,
		otelresource.WithTelemetrySDK(),
		otelresource.WithHost(),
		otelresource.WithAttributes(attrs...),
		otelresource.WithFromEnv(),
	)
}
