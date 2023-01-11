package session

import (
	"testing"
	"time"

	"github.com/google/uuid"
	v1 "go.viam.com/api/robot/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestNew(t *testing.T) {
	ownerID := "owner1"
	remoteAddr := "rem"
	localAddr := "loc"
	info := &v1.PeerConnectionInfo{
		Type:          v1.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
		RemoteAddress: &remoteAddr,
		LocalAddress:  &localAddr,
	}
	now := time.Now()
	dur := time.Second
	someFunc := func(id uuid.UUID, resourceName resource.Name) {
	}
	sess1 := New(ownerID, info, dur, someFunc)
	sess2 := New(ownerID+"other", info, dur, someFunc)
	test.That(t, sess2.CheckOwnerID(ownerID), test.ShouldBeFalse)
	test.That(t, sess2.CheckOwnerID(ownerID+"other"), test.ShouldBeTrue)

	test.That(t, sess1.ID(), test.ShouldNotEqual, uuid.Nil)
	test.That(t, sess1.ID(), test.ShouldNotEqual, sess2.ID())
	test.That(t, sess1.CheckOwnerID(ownerID), test.ShouldBeTrue)
	test.That(t, sess1.CheckOwnerID(ownerID+"other"), test.ShouldBeFalse)
	test.That(t, sess1.Active(now), test.ShouldBeTrue)
	time.Sleep(2 * dur)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeFalse)
	sess1.Heartbeat()
	test.That(t, sess1.Active(time.Now()), test.ShouldBeTrue)
	test.That(t, sess1.PeerConnectionInfo(), test.ShouldResembleProto, info)
	test.That(t, sess1.HeartbeatWindow(), test.ShouldEqual, dur)
	test.That(t, sess1.Deadline().After(now), test.ShouldBeTrue)
	test.That(t, sess1.Deadline().Before(now.Add(2*dur)), test.ShouldBeFalse)
}

func TestNewWithID(t *testing.T) {
	ownerID := "owner1"
	remoteAddr := "rem"
	localAddr := "loc"
	info := &v1.PeerConnectionInfo{
		Type:          v1.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
		RemoteAddress: &remoteAddr,
		LocalAddress:  &localAddr,
	}
	now := time.Now()
	dur := time.Second
	someFunc := func(id uuid.UUID, resourceName resource.Name) {
	}

	id1 := uuid.New()
	id2 := uuid.New()

	sess1 := NewWithID(id1, ownerID, info, dur, someFunc)
	sess2 := NewWithID(id2, ownerID+"other", info, dur, someFunc)
	test.That(t, sess2.CheckOwnerID(ownerID), test.ShouldBeFalse)
	test.That(t, sess2.CheckOwnerID(ownerID+"other"), test.ShouldBeTrue)

	test.That(t, sess1.ID(), test.ShouldEqual, id1)
	test.That(t, sess2.ID(), test.ShouldEqual, id2)
	test.That(t, sess1.CheckOwnerID(ownerID), test.ShouldBeTrue)
	test.That(t, sess1.CheckOwnerID(ownerID+"other"), test.ShouldBeFalse)
	test.That(t, sess1.Active(now), test.ShouldBeTrue)
	time.Sleep(2 * dur)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeFalse)
	sess1.Heartbeat()
	test.That(t, sess1.Active(time.Now()), test.ShouldBeTrue)
	test.That(t, sess1.PeerConnectionInfo(), test.ShouldResembleProto, info)
	test.That(t, sess1.HeartbeatWindow(), test.ShouldEqual, dur)
	test.That(t, sess1.Deadline().After(now), test.ShouldBeTrue)
	test.That(t, sess1.Deadline().Before(now.Add(2*dur)), test.ShouldBeFalse)
}
