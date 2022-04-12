package operation

import (
	"context"
	"testing"

	"go.viam.com/test"
)

func TestBasic(t *testing.T) {
	ctx := context.Background()
	h := NewManager()
	o := Get(ctx)
	test.That(t, o, test.ShouldBeNil)

	test.That(t, len(h.All()), test.ShouldEqual, 0)

	func() {
		ctx2, cleanup := h.Create(ctx, "1", nil)
		defer cleanup()

		test.That(t, func() { h.Create(ctx2, "b", nil) }, test.ShouldPanic)

		o := Get(ctx2)
		test.That(t, o, test.ShouldNotBeNil)
		test.That(t, o.ID.String(), test.ShouldNotEqual, "")
		test.That(t, len(h.All()), test.ShouldEqual, 1)
		test.That(t, h.All()[0].ID, test.ShouldEqual, o.ID)
		test.That(t, h.Find(o.ID).ID, test.ShouldEqual, o.ID)
		test.That(t, h.FindString(o.ID.String()).ID, test.ShouldEqual, o.ID)
	}()

	test.That(t, len(h.All()), test.ShouldEqual, 0)

	func() {
		ctx2, cleanup2 := h.Create(ctx, "a", nil)
		defer cleanup2()

		ctx3, cleanup3 := h.Create(ctx, "b", nil)
		defer cleanup3()

		CancelOtherWithLabel(ctx2, "foo")
		CancelOtherWithLabel(ctx3, "foo")
		CancelOtherWithLabel(ctx, "foo")

		test.That(t, ctx3.Err(), test.ShouldBeNil)
		test.That(t, ctx2.Err(), test.ShouldNotBeNil)
	}()
}
