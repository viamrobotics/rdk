package limo

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestLimoBaseConstructor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := resource.Dependencies{}
	// expectedWidth := float64(defaultBaseTreadMm) * 0.001
	expectedWidth := defaultBaseWidthM
	expectedTurningRadius := minTurningRadiusM // only for ackerman

	c := make(chan []uint8, 100)

	_, err := createLimoBase(ctx, deps, resource.Config{ConvertedAttributes: &Config{}}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := &Config{
		DriveMode: "ackermann",
		TestChan:  c,
	}

	lb, err := createLimoBase(ctx, deps, resource.Config{ConvertedAttributes: cfg}, logger)
	test.That(t, err, test.ShouldBeNil)
	props, err := lb.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, props.WidthMeters, test.ShouldEqual, expectedWidth)
	test.That(t, props.TurningRadiusMeters, test.ShouldEqual, expectedTurningRadius)
	lb.Close(ctx)

	cfg = &Config{
		DriveMode: "differential",
		TestChan:  c,
	}
	lb, err = createLimoBase(context.Background(), deps, resource.Config{ConvertedAttributes: cfg}, logger)
	test.That(t, err, test.ShouldBeNil)
	props, err = lb.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.WidthMeters, test.ShouldEqual, expectedWidth)
	test.That(t, props.TurningRadiusMeters, test.ShouldEqual, 0) // not ackerman, so zero
	lb.Close(ctx)

	cfg = &Config{
		DriveMode: "omni",
		TestChan:  c,
	}
	lb, err = createLimoBase(context.Background(), deps, resource.Config{ConvertedAttributes: cfg}, logger)
	test.That(t, err, test.ShouldBeNil)
	props, err = lb.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.WidthMeters, test.ShouldEqual, expectedWidth)
	test.That(t, props.TurningRadiusMeters, test.ShouldEqual, 0) // not ackerman, so zero
	lb.Close(ctx)
}
