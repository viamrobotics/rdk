package server

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.uber.org/multierr"

	"go.viam.com/rdk/config"
)

// otlpMetricsEnabled reports whether the standard OTEL_ environment variables name an OTLP
// endpoint for metrics.
func otlpMetricsEnabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// newMeterProvider exports metrics, including Go runtime metrics, to the OTLP endpoint that the
// OTEL_ environment variables configure.
func newMeterProvider(ctx context.Context, partID string) (*sdkmetric.MeterProvider, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName("rdk"), semconv.ServiceNamespace("viam.com")}
	if config.Version != "" {
		attrs = append(attrs, semconv.ServiceVersion(config.Version))
	}
	if partID != "" {
		attrs = append(attrs, attribute.String("viam.part.id", partID))
	}
	res, err := otelresource.New(ctx,
		otelresource.WithTelemetrySDK(),
		otelresource.WithHost(),
		otelresource.WithAttributes(attrs...),
		otelresource.WithFromEnv(),
	)
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
	return provider, nil
}
