package fake

import (
	"context"
	"testing"

	"go.viam.com/test"
)


func TestEncoder(t *testing.T) {
	ctx := context.Background()

	e := &Encoder{}

	// Get and set position
	pos, err := e.GetTicksCount(ctx, nil)
	test.That(t, pos, test.ShouldEqual, 0)
	test.That(t, err, test.ShouldBeNil)

	err = e.SetPosition(ctx, 1)
	test.That(t, err, test.ShouldBeNil)

	pos, err = e.GetTicksCount(ctx, nil)
	test.That(t, pos, test.ShouldEqual, 1)
	test.That(t, err, test.ShouldBeNil)

	// ResetToZero
	err = e.ResetToZero(ctx, 0, nil)
	test.That(t, err, test.ShouldBeNil)

	pos, err = e.GetTicksCount(ctx, nil)
	test.That(t, pos, test.ShouldEqual, 0)
	test.That(t, err, test.ShouldBeNil)

	// Set Speed
	err = e.SetSpeed(ctx, 1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, e.speed, test.ShouldEqual, 1)
}