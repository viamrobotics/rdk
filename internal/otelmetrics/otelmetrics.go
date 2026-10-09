// Package otelmetrics owns viam-server's OpenTelemetry MeterProvider and the readers that export
// from it: one for FTDC and one for OTLP.
package otelmetrics

import (
	"context"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	otelresource "go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.uber.org/multierr"

	"go.viam.com/rdk/ftdc"
)

// Attribute keys shared by viam metrics.
const (
	ResourceAPIKey  = attribute.Key("viam.resource.api")
	ResourceNameKey = attribute.Key("viam.resource.name")
)

// OTLPEnabled reports whether an OTLP endpoint is set through the standard OTEL_ environment variables.
func OTLPEnabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// Config selects the readers attached to the MeterProvider and identifies the machine its
// metrics describe.
type Config struct {
	PartID     string
	MachineID  string
	LocationID string
	OrgID      string
	Version    string
	FTDC       bool
	OTLP       bool
}

// Metrics is the process-wide MeterProvider. A nil *Metrics records nothing.
type Metrics struct {
	provider *sdkmetric.MeterProvider
	ftdc     *ftdcStatser
}

// New builds the MeterProvider and starts Go runtime metrics on it. With OTLP set, it also becomes
// the global provider so that otelgrpc metrics reach the OTLP exporter.
func New(ctx context.Context, cfg Config) (*Metrics, error) {
	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	m := &Metrics{}
	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	if cfg.FTDC {
		m.ftdc = newFTDCStatser()
		opts = append(opts, sdkmetric.WithReader(m.ftdc.reader))
	}
	if cfg.OTLP {
		exporter, err := otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, err
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)))
	}
	m.provider = sdkmetric.NewMeterProvider(opts...)

	if err := runtime.Start(runtime.WithMeterProvider(m.provider)); err != nil {
		return nil, multierr.Combine(err, m.provider.Shutdown(ctx))
	}
	if cfg.OTLP {
		otel.SetMeterProvider(m.provider)
	}
	return m, nil
}

// MeterProvider returns the provider rdk instruments record into.
func (m *Metrics) MeterProvider() metric.MeterProvider {
	if m == nil {
		return noop.NewMeterProvider()
	}
	return m.provider
}

// FTDCStatser returns the statser that writes these metrics into FTDC, or nil when FTDC is off.
// Register it with an empty name: its keys are fully qualified.
func (m *Metrics) FTDCStatser() ftdc.Statser {
	if m == nil || m.ftdc == nil {
		return nil
	}
	return m.ftdc
}

// MapFTDC writes the metric named metricName into FTDC through fn in place of the generic naming.
func (m *Metrics) MapFTDC(metricName string, fn FTDCKeyFunc) {
	if m == nil || m.ftdc == nil {
		return
	}
	m.ftdc.mapMetric(metricName, fn)
}

// Shutdown flushes and stops every reader.
func (m *Metrics) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	return m.provider.Shutdown(ctx)
}

// newResource leaves the viam attributes schemaless so they merge with whatever semconv schema
// the SDK's detectors use; two different schema URLs fail the merge.
func newResource(ctx context.Context, cfg Config) (*otelresource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName("rdk"), semconv.ServiceNamespace("viam.com")}
	if cfg.Version != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.Version))
	}
	for key, value := range map[string]string{
		"viam.part.id":     cfg.PartID,
		"viam.machine.id":  cfg.MachineID,
		"viam.location.id": cfg.LocationID,
		"viam.org.id":      cfg.OrgID,
	} {
		if value != "" {
			attrs = append(attrs, attribute.String(key, value))
		}
	}
	return otelresource.New(ctx,
		otelresource.WithTelemetrySDK(),
		otelresource.WithHost(),
		otelresource.WithAttributes(attrs...),
		otelresource.WithFromEnv(),
	)
}
