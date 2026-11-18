package server

import (
	"context"
	"net"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.viam.com/test"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

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

func TestMetricViewsDropServerDuration(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(metricViews()...))
	defer func() { test.That(t, provider.Shutdown(context.Background()), test.ShouldBeNil) }()

	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler(otelgrpc.WithMeterProvider(provider))))
	healthpb.RegisterHealthServer(server, health.NewServer())
	go server.Serve(listener)
	defer server.Stop()

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return listener.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, conn.Close(), test.ShouldBeNil) }()
	_, err = healthpb.NewHealthClient(conn).Check(context.Background(), &healthpb.HealthCheckRequest{})
	test.That(t, err, test.ShouldBeNil)

	var rm metricdata.ResourceMetrics
	test.That(t, reader.Collect(context.Background(), &rm), test.ShouldBeNil)
	var names []string
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			names = append(names, m.Name)
		}
	}
	test.That(t, names, test.ShouldContain, "rpc.server.request.size")
	test.That(t, names, test.ShouldNotContain, "rpc.server.duration")
}
