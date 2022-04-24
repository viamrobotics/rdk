package operation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.viam.com/test"
)

func TestNBCallManager1(t *testing.T) {
	cm := NBCallManager{}
	ctx := context.Background()

	t.Run("done1", func(t *testing.T) {
		done := int32(0)
		cancelled := int32(0)
		
		op := cm.NewTimed(ctx, 10 * time.Millisecond, func() { atomic.StoreInt32(&done, 1) }, func() { atomic.StoreInt32(&cancelled, 1) })
		op.Block(ctx)
		test.That(t, done, test.ShouldEqual, 1)
		test.That(t, cancelled, test.ShouldEqual, 0)
	})

	t.Run("cancel1", func(t *testing.T) {
		done := int32(0)
		cancelled := int32(0)
		
		op := cm.NewTimed(ctx, 10 * time.Second, func() { atomic.StoreInt32(&done, 1) }, func() { atomic.StoreInt32(&cancelled, 1) })
		op.Cancel()
		time.Sleep(10*time.Millisecond) // make sure cancellation has time to finish
		test.That(t, done, test.ShouldEqual, 0)
		test.That(t, cancelled, test.ShouldEqual, 1)
		op.Block(ctx) // just testing this doesn't hang
	})

	t.Run("cancel2", func(t *testing.T) {
		done := int32(0)
		cancelled := int32(0)
		
		_ = cm.NewTimed(ctx, 10 * time.Second, func() { atomic.StoreInt32(&done, 1) }, func() { atomic.StoreInt32(&cancelled, 1) })
		cm.CancelRunning()
		time.Sleep(10*time.Millisecond) // make sure cancellation has time to finish
		test.That(t, done, test.ShouldEqual, 0)
		test.That(t, cancelled, test.ShouldEqual, 1)
	})



}
