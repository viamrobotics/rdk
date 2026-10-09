package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/internal/otelmetrics"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestResourceStateFTDCKeys(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	metrics, err := otelmetrics.New(ctx, otelmetrics.Config{FTDC: true})
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, metrics.Shutdown(ctx), test.ShouldBeNil) })

	cfg := &config.Config{Components: []resource.Config{
		{Name: "good", Model: fake.Model, API: arm.API, ConvertedAttributes: &fake.Config{}},
		{Name: "bad", Model: resource.DefaultModelFamily.WithModel("missing"), API: arm.API},
	}}
	setupLocalRobot(t, ctx, cfg, logger, WithMetrics(metrics))

	stats := metrics.FTDCStatser().Stats().(map[string]float64)
	for name, want := range map[string]resource.Availability{
		arm.Named("good").String(): resource.AvailabilityReady,
		arm.Named("bad").String():  resource.AvailabilityUnhealthy,
	} {
		got, ok := stats[name+".State"]
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, got, test.ShouldEqual, float64(want))
	}
}
