package fake

import (
	"context"
	"testing"
	"time"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"
)

func TestEncoder(t *testing.T) {
	ctx := context.Background()

	e := &Encoder{}

	// Get and set position
	t.Run("get and set position", func(t *testing.T) {
		pos, err := e.GetTicksCount(ctx, nil)
		test.That(t, pos, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeNil)

		err = e.SetPosition(ctx, 1)
		test.That(t, err, test.ShouldBeNil)

		pos, err = e.GetTicksCount(ctx, nil)
		test.That(t, pos, test.ShouldEqual, 1)
		test.That(t, err, test.ShouldBeNil)
	})

	// ResetToZero
	t.Run("reset to zero", func(t *testing.T) {
		err := e.ResetToZero(ctx, 0, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, err := e.GetTicksCount(ctx, nil)
		test.That(t, pos, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeNil)
	})

	// Set Speed
	t.Run("set speed", func(t *testing.T) {
		err := e.SetSpeed(ctx, 1)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, e.speed, test.ShouldEqual, 1)
	})

	// Start with default update rate
	t.Run("start default update rate", func(t *testing.T) {
		err := e.SetSpeed(ctx, 0)
		test.That(t, err, test.ShouldBeNil)

		e.Start(ctx, func(){})

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(t, e.updateRate, test.ShouldEqual, 100)
		})

		err = e.SetSpeed(ctx, 600)
		test.That(t, err, test.ShouldBeNil)

		time.Sleep(200 * time.Millisecond)
		pos, err := e.GetTicksCount(ctx, nil)
		test.That(t, pos, test.ShouldBeGreaterThan, 0)
		test.That(t, err, test.ShouldBeNil)
	})

}
