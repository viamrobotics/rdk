package operation

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/session"
)

func TestBasic(t *testing.T) {
	ctx := context.Background()

	logger := logging.NewTestLogger(t)
	h := NewManager(logger)
	o := Get(ctx)
	test.That(t, o, test.ShouldBeNil)

	test.That(t, len(h.All()), test.ShouldEqual, 0)

	func() {
		ctx2, cleanup := h.Create(ctx, "1", nil)
		defer cleanup()

		test.That(t, func() { h.Create(ctx2, "b", nil) }, test.ShouldPanic)

		o := Get(ctx2)
		test.That(t, o, test.ShouldNotBeNil)
		test.That(t, len(o.myManager.ops), test.ShouldNotEqual, 0)
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

	func() {
		ctx4, cleanup4 := h.Create(ctx, "/proto.rpc.webrtc.v1.SignalingService/Answer", nil)
		defer cleanup4()
		ctx5, cleanup5 := h.Create(ctx, "/viam.robot.v1.RobotService/StreamStatus", nil)
		defer cleanup5()

		test.That(t, ctx4.Value(opidKey), test.ShouldBeNil)
		test.That(t, ctx5.Value(opidKey), test.ShouldBeNil)

		ctx6, cleanup6 := h.Create(ctx, "/viam.robot.v1.RobotService/", nil)
		defer cleanup6()
		o6 := Get(ctx6)
		test.That(t, len(o6.myManager.ops), test.ShouldEqual, 1)
	}()
}

func TestCreateWithSession(t *testing.T) {
	ctx := context.Background()

	logger := logging.NewTestLogger(t)
	manager := NewManager(logger)

	op1Ctx, cleanup := manager.Create(ctx, "foo", nil)
	op1 := Get(op1Ctx)
	test.That(t, op1.SessionID, test.ShouldEqual, uuid.Nil)
	cleanup()

	sess1 := session.New(ctx, "someone", 0, nil)
	sess1Ctx := session.ToContext(ctx, sess1)

	op2Ctx, cleanup := manager.Create(sess1Ctx, "foo", nil)
	op2 := Get(op2Ctx)
	test.That(t, op2.SessionID, test.ShouldEqual, sess1.ID())
	cleanup()
}
