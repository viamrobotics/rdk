package otelmetrics

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.viam.com/test"
)

func TestNewResource(t *testing.T) {
	res, err := newResource(context.Background(), "part-123", "v0.0.1")
	test.That(t, err, test.ShouldBeNil)

	set := res.Set()
	for key, want := range map[attribute.Key]string{
		"service.name":      "rdk",
		"service.namespace": "viam.com",
		"service.version":   "v0.0.1",
		"viam.part.id":      "part-123",
	} {
		got, ok := set.Value(key)
		test.That(t, ok, test.ShouldBeTrue)
		test.That(t, got.AsString(), test.ShouldEqual, want)
	}
	_, ok := set.Value("host.name")
	test.That(t, ok, test.ShouldBeTrue)

	t.Setenv("OTEL_SERVICE_NAME", "from-env")
	res, err = newResource(context.Background(), "", "")
	test.That(t, err, test.ShouldBeNil)
	got, _ := res.Set().Value("service.name")
	test.That(t, got.AsString(), test.ShouldEqual, "from-env")
}
