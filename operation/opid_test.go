package operation

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestBasic(t *testing.T) {
	ctx := context.Background()
	o := Get(ctx)
	test.That(t, o, test.ShouldBeNil)

	test.That(t, len(CurrentOps()), test.ShouldEqual, 0)

	func() {
		ctx2, cleanup := Create(ctx, "1", nil)
		defer cleanup()

		test.That(t, func() { Create(ctx2, "b", nil) }, test.ShouldPanic)

		o := Get(ctx2)
		test.That(t, o, test.ShouldNotBeNil)
		test.That(t, o.ID.String(), test.ShouldNotEqual, "")
		test.That(t, len(CurrentOps()), test.ShouldEqual, 1)
		test.That(t, CurrentOps()[0].ID, test.ShouldEqual, o.ID)
		test.That(t, FindOp(o.ID).ID, test.ShouldEqual, o.ID)
		test.That(t, FindOpString(o.ID.String()).ID, test.ShouldEqual, o.ID)
	}()

	test.That(t, len(CurrentOps()), test.ShouldEqual, 0)

	func() {
		ctx2, cleanup2 := Create(ctx, "a", nil)
		defer cleanup2()

		ctx3, cleanup3 := Create(ctx, "b", nil)
		defer cleanup3()

		Get(ctx2).CancelOtherWithLabel("foo")
		Get(ctx3).CancelOtherWithLabel("foo")

		test.That(t, ctx3.Err(), test.ShouldBeNil)
		test.That(t, ctx2.Err(), test.ShouldNotBeNil)
	}()
}
