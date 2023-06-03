package limo

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/resource"
)

func TestLimoBaseConstructor(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	deps := resource.Dependencies{}

	c := make(chan []uint8, 100)

	_, err := createLimoBase(context.Background(), deps, resource.Config{ConvertedAttributes: &Config{}}, logger)
	test.That(t, err, test.ShouldNotBeNil)

	cfg := &Config{
		DriveMode: "ackermann",
		TestChan:  c,
	}

	lb, err := createLimoBase(context.Background(), deps, resource.Config{ConvertedAttributes: cfg}, logger)
	test.That(t, err, test.ShouldBeNil)
	props, err := lb.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, props[base.WidthM], test.ShouldEqual, 172)
	lb.Close(ctx)
}
