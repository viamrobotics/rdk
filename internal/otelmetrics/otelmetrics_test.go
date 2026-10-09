package otelmetrics

import (
	"context"
	"testing"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.viam.com/test"
)

func TestNewResource(t *testing.T) {
	res, err := newResource(context.Background(), Config{
		PartID: "part-123", MachineID: "machine-1", LocationID: "location-1", OrgID: "org-1", Version: "v0.0.1",
	})
	test.That(t, err, test.ShouldBeNil)

	set := res.Set()
	for key, want := range map[attribute.Key]string{
		"service.name":      "rdk",
		"service.namespace": "viam.com",
		"service.version":   "v0.0.1",
		"viam.part.id":      "part-123",
		"viam.machine.id":   "machine-1",
		"viam.location.id":  "location-1",
		"viam.org.id":       "org-1",
	} {
		got, ok := set.Value(key)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, got.AsString(), test.ShouldEqual, want)
	}
	_, ok := set.Value("host.name")
	test.That(t, ok, test.ShouldBeTrue)

	t.Setenv("OTEL_SERVICE_NAME", "from-env")
	res, err = newResource(context.Background(), Config{})
	test.That(t, err, test.ShouldBeNil)
	got, _ := res.Set().Value("service.name")
	test.That(t, got.AsString(), test.ShouldEqual, "from-env")
}

func TestFTDCStatser(t *testing.T) {
	ctx := context.Background()
	m, err := New(ctx, Config{FTDC: true})
	test.That(t, err, test.ShouldBeNil)
	t.Cleanup(func() { test.That(t, m.Shutdown(ctx), test.ShouldBeNil) })

	rdkMeter := m.MeterProvider().Meter("go.viam.com/rdk/test")
	mapped, err := rdkMeter.Int64Counter("viam.mapped")
	test.That(t, err, test.ShouldBeNil)
	generic, err := rdkMeter.Int64Counter("viam.generic")
	test.That(t, err, test.ShouldBeNil)
	latency, err := rdkMeter.Float64Histogram("viam.latency")
	test.That(t, err, test.ShouldBeNil)
	rpc, err := m.MeterProvider().Meter(otelgrpc.ScopeName).Int64Counter("viam.rpc")
	test.That(t, err, test.ShouldBeNil)

	m.MapFTDC("viam.mapped", func(p FTDCPoint, add func(string, float64)) {
		name, _ := p.Attrs.Value("name")
		add(name.AsString()+".legacy", p.Value*1000)
		add("shared", p.Value)
	})
	m.MapFTDC("viam.latency", func(p FTDCPoint, add func(string, float64)) {
		add("shared", float64(p.Count))
	})
	attrs := metric.WithAttributes(attribute.String("name", "foo"))
	mapped.Add(ctx, 2, attrs)
	generic.Add(ctx, 3, attrs)
	latency.Record(ctx, 0.5, attrs)
	rpc.Add(ctx, 4)

	generic.Add(ctx, 1, metric.WithAttributes(attribute.String("name", "bar")))
	other, err := rdkMeter.Float64Histogram("viam.other")
	test.That(t, err, test.ShouldBeNil)
	other.Record(ctx, 0.5, attrs)

	stats := m.FTDCStatser().Stats().(map[string]float64)
	for key, want := range map[string]float64{
		"foo.legacy":           2000,
		"shared":               3,
		"viam.generic.foo":     3,
		"viam.generic.bar":     1,
		"viam.other.foo.count": 1,
		"viam.other.foo.sum":   0.5,
	} {
		test.That(t, stats[key], test.ShouldEqual, want)
	}
	test.That(t, stats, test.ShouldNotContainKey, "viam.rpc")
	test.That(t, stats["go.goroutine.count"], test.ShouldBeGreaterThan, 0)
}
