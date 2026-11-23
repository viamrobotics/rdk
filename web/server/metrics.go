package server

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

// OTLP endpoint variables that enable metrics export, most specific first.
var otlpMetricsEndpointEnvs = []string{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"}

// setupMetricsExport installs a global OTel MeterProvider that exports over OTLP/gRPC when an OTLP
// endpoint variable is set, so instrumented libraries such as otelgrpc record metrics. Otherwise the
// global provider stays a no-op and the returned shutdown function does nothing. The exporter reads
// the remaining standard OTEL_EXPORTER_OTLP_* variables (insecure, headers, timeout), the reader
// honors OTEL_METRIC_EXPORT_INTERVAL, and OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES override
// the resource attributes.
func setupMetricsExport(
	ctx context.Context, cfg *config.Config, logger logging.Logger,
) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }

	envName, endpoint := otlpMetricsEndpoint()
	if endpoint == "" {
		return noop, nil
	}
	// The exporter accepts an endpoint without a host, such as "localhost:4317" (parsed as scheme
	// "localhost"), and then fails every export.
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return noop, fmt.Errorf("%s=%q must be a URL such as http://localhost:4317", envName, endpoint)
	}
	exporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return noop, err
	}

	attrs := []attribute.KeyValue{attribute.String("service.name", "viam-server")}
	if cfg.Cloud != nil && cfg.Cloud.ID != "" {
		attrs = append(attrs, attribute.String("viam.part.id", cfg.Cloud.ID))
	}
	// WithFromEnv comes last so the environment overrides the defaults.
	res, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
	)
	if err != nil {
		return noop, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(provider)
	logger.Infow("Exporting OTLP metrics", "endpoint", u.Redacted(), "env", envName)
	return provider.Shutdown, nil
}

func otlpMetricsEndpoint() (envName, endpoint string) {
	for _, envName := range otlpMetricsEndpointEnvs {
		if endpoint := os.Getenv(envName); endpoint != "" {
			return envName, endpoint
		}
	}
	return "", ""
}
