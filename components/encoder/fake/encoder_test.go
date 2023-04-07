package fake

import (
	"context"
	"testing"

	pb "go.viam.com/api/component/encoder/v1"
	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/encoder"
)

func TestEncoder(t *testing.T) {
	ctx := context.Background()

	e := &Encoder{}

	// Get and set position
	t.Run("get and set position", func(t *testing.T) {
		pos, _, err := e.GetPosition(ctx, nil, nil)
		test.That(t, pos, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeNil)

		err = e.SetPosition(ctx, 1)
		test.That(t, err, test.ShouldBeNil)

		pos, _, err = e.GetPosition(ctx, nil, nil)
		test.That(t, pos, test.ShouldEqual, 1)
		test.That(t, err, test.ShouldBeNil)
	})

	// Reset
	t.Run("reset to zero", func(t *testing.T) {
		err := e.ResetPosition(ctx, nil)
		test.That(t, err, test.ShouldBeNil)

		pos, _, err := e.GetPosition(ctx, nil, nil)
		test.That(t, pos, test.ShouldEqual, 0)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("specify a type", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			ticks, positionType, err := e.GetPosition(context.Background(), pb.PositionType_POSITION_TYPE_TICKS_COUNT.Enum(), nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, ticks, test.ShouldEqual, 0)
			test.That(tb, positionType, test.ShouldEqual, pb.PositionType_POSITION_TYPE_UNSPECIFIED)
		})
	})
	t.Run("get properties", func(t *testing.T) {
		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			props, err := e.GetProperties(ctx, nil)
			test.That(tb, err, test.ShouldBeNil)
			test.That(tb, props[encoder.TicksCountSupported], test.ShouldBeFalse)
			test.That(tb, props[encoder.AngleDegreesSupported], test.ShouldBeFalse)
		})
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

		e.Start(ctx)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			test.That(t, e.updateRate, test.ShouldEqual, 100)
		})

		err = e.SetSpeed(ctx, 600)
		test.That(t, err, test.ShouldBeNil)

		testutils.WaitForAssertion(t, func(tb testing.TB) {
			tb.Helper()
			pos, _, err := e.GetPosition(ctx, nil, nil)
			test.That(tb, pos, test.ShouldBeGreaterThan, 0)
			test.That(tb, err, test.ShouldBeNil)
		})
	})
}
