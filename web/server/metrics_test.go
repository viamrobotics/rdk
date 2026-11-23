package server

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
)

func TestSetupMetricsExport(t *testing.T) {
	for _, tc := range []struct {
		name            string
		metricsEndpoint string
		endpoint        string
		ok              bool
	}{
		{"disabled", "", "", true},
		{"url", "http://localhost:4317", "", true},
		{"url without port", "https://collector.example.com", "", true},
		{"generic endpoint", "", "http://localhost:4317", true},
		{"metrics endpoint wins", "http://localhost:4317", "localhost:4317", true},
		{"host:port", "localhost:4317", "", false},
		{"generic host:port", "", "localhost:4317", false},
		{"url without host", "http://", "", false},
		{"port without host", ":4317", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", tc.metricsEndpoint)
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", tc.endpoint)
			before := otel.GetMeterProvider()

			shutdown, err := setupMetricsExport(context.Background(), &config.Config{}, logging.NewTestLogger(t))
			if tc.ok {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
			}

			enabled := tc.ok && (tc.metricsEndpoint != "" || tc.endpoint != "")
			test.That(t, otel.GetMeterProvider() != before, test.ShouldEqual, enabled)
			if !enabled {
				test.That(t, shutdown(context.Background()), test.ShouldBeNil)
				return
			}
			otel.SetMeterProvider(before)
			// Nothing listens at the endpoints, so skip the final export rather than wait for it to
			// time out.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			shutdown(ctx)
		})
	}
}
