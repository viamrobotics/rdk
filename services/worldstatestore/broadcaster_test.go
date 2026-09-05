package worldstatestore

import (
	"testing"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/test"
)

func added(uuid string) TransformChange {
	return TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
		Transform:  &commonpb.Transform{Uuid: []byte(uuid)},
	}
}

func updated(uuid string) TransformChange {
	return TransformChange{
		ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
		Transform:  &commonpb.Transform{Uuid: []byte(uuid)},
	}
}

func TestBroadcasterFanOut(t *testing.T) {
	b := NewTransformChangeBroadcaster()
	defer b.Close()

	ch1, unsub1 := b.Subscribe(4)
	ch2, unsub2 := b.Subscribe(4)
	defer unsub1()
	defer unsub2()

	b.Broadcast(added("a"))

	got1 := <-ch1
	got2 := <-ch2
	test.That(t, string(got1.Transform.Uuid), test.ShouldEqual, "a")
	test.That(t, string(got2.Transform.Uuid), test.ShouldEqual, "a")
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	b := NewTransformChangeBroadcaster()
	defer b.Close()

	ch, unsub := b.Subscribe(4)
	unsub()

	// Channel is closed after unsubscribe.
	_, ok := <-ch
	test.That(t, ok, test.ShouldBeFalse)

	// Broadcasting after unsubscribe is a no-op (does not panic).
	b.Broadcast(added("a"))
}

func TestBroadcasterStructuralOverflowDisconnects(t *testing.T) {
	b := NewTransformChangeBroadcaster()
	defer b.Close()

	ch, unsub := b.Subscribe(1)
	defer unsub()

	b.Broadcast(added("a")) // fills the buffer
	b.Broadcast(added("b")) // overflow on a structural change -> subscriber disconnected

	first := <-ch
	test.That(t, string(first.Transform.Uuid), test.ShouldEqual, "a")

	_, ok := <-ch // channel closed because the subscriber was disconnected
	test.That(t, ok, test.ShouldBeFalse)
}

func TestBroadcasterUpdatedOverflowDrops(t *testing.T) {
	b := NewTransformChangeBroadcaster()
	defer b.Close()

	ch, unsub := b.Subscribe(1)
	defer unsub()

	b.Broadcast(updated("a")) // fills the buffer
	b.Broadcast(updated("b")) // dropped, subscriber stays connected

	first := <-ch
	test.That(t, string(first.Transform.Uuid), test.ShouldEqual, "a")

	// Still connected: a subsequent broadcast is delivered.
	b.Broadcast(updated("c"))
	next := <-ch
	test.That(t, string(next.Transform.Uuid), test.ShouldEqual, "c")
}

func TestBroadcasterClose(t *testing.T) {
	b := NewTransformChangeBroadcaster()

	ch, _ := b.Subscribe(4)
	b.Close()

	_, ok := <-ch
	test.That(t, ok, test.ShouldBeFalse)

	// Subscribing after close returns an already-closed channel.
	ch2, _ := b.Subscribe(4)
	_, ok = <-ch2
	test.That(t, ok, test.ShouldBeFalse)
}
