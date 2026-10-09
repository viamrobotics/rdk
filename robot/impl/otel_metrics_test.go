package robotimpl

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestResourceStateMetric(t *testing.T) {
	ctx := context.Background()
	reader := sdkmetric.NewManualReader()
	cfg := &config.Config{Components: []resource.Config{
		{Name: "arm1", Model: fake.Model, API: arm.API, ConvertedAttributes: &fake.Config{}},
	}}
	setupLocalRobot(t, ctx, cfg, logging.NewTestLogger(t),
		WithMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))))

	var rm metricdata.ResourceMetrics
	test.That(t, reader.Collect(ctx, &rm), test.ShouldBeNil)
	want := attribute.NewSet(
		attribute.String("viam.resource.api", arm.API.String()),
		attribute.String("viam.resource.name", "arm1"),
		attribute.String("viam.resource.state", resource.NodeStateReady.String()),
	)
	var found bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if gauge, ok := m.Data.(metricdata.Gauge[int64]); ok && m.Name == "viam.resource.state" {
				for _, dp := range gauge.DataPoints {
					found = found || dp.Attributes.Equals(&want)
				}
			}
		}
	}
	test.That(t, found, test.ShouldBeTrue)
}
