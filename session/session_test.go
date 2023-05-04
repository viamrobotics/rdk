package session

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	v1 "go.viam.com/api/robot/v1"
	"go.viam.com/test"
	"google.golang.org/grpc/peer"

	"go.viam.com/rdk/resource"
)

func TestNew(t *testing.T) {
	ownerID := "owner1"
	remoteAddr := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5}
	remoteAddrStr := remoteAddr.String()
	info := &v1.PeerConnectionInfo{
		Type:          v1.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
		RemoteAddress: &remoteAddrStr,
		LocalAddress:  nil,
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: remoteAddr,
	})
	now := time.Now()
	dur := time.Second
	someFunc := func(id uuid.UUID, resourceName resource.Name) {
	}
	sess1 := New(ctx, ownerID, dur, someFunc)
	test.That(t, sess1.PeerConnectionInfo(), test.ShouldResembleProto, info)
	sess2 := New(ctx, ownerID+"other", dur, someFunc)
	test.That(t, sess2.PeerConnectionInfo(), test.ShouldResembleProto, info)
	test.That(t, sess2.CheckOwnerID(ownerID), test.ShouldBeFalse)
	test.That(t, sess2.CheckOwnerID(ownerID+"other"), test.ShouldBeTrue)

	test.That(t, sess1.ID(), test.ShouldNotEqual, uuid.Nil)
	test.That(t, sess1.ID(), test.ShouldNotEqual, sess2.ID())
	test.That(t, sess1.CheckOwnerID(ownerID), test.ShouldBeTrue)
	test.That(t, sess1.CheckOwnerID(ownerID+"other"), test.ShouldBeFalse)
	test.That(t, sess1.Active(now), test.ShouldBeTrue)
	time.Sleep(2 * dur)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeFalse)
	remoteAddr = &net.TCPAddr{IP: net.IPv4(1, 2, 5, 4), Port: 6}
	remoteAddrStr = remoteAddr.String()
	info = &v1.PeerConnectionInfo{
		Type:          v1.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
		RemoteAddress: &remoteAddrStr,
	}
	ctx = peer.NewContext(context.Background(), &peer.Peer{
		Addr: remoteAddr,
	})
	sess1.Heartbeat(ctx)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeTrue)
	test.That(t, sess1.PeerConnectionInfo(), test.ShouldResembleProto, info)
	test.That(t, sess1.HeartbeatWindow(), test.ShouldEqual, dur)
	test.That(t, sess1.Deadline().After(now), test.ShouldBeTrue)
	test.That(t, sess1.Deadline().Before(now.Add(2*dur)), test.ShouldBeFalse)
}

func TestNewWithID(t *testing.T) {
	ownerID := "owner1"
	remoteAddr := &net.TCPAddr{IP: net.IPv4(1, 2, 3, 4), Port: 5}
	remoteAddrStr := remoteAddr.String()
	info := &v1.PeerConnectionInfo{
		Type:          v1.PeerConnectionType_PEER_CONNECTION_TYPE_GRPC,
		RemoteAddress: &remoteAddrStr,
		LocalAddress:  nil,
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{
		Addr: remoteAddr,
	})
	now := time.Now()
	dur := time.Second
	someFunc := func(id uuid.UUID, resourceName resource.Name) {
	}

	id1 := uuid.New()
	id2 := uuid.New()

	sess1 := NewWithID(ctx, id1, ownerID, dur, someFunc)
	sess2 := NewWithID(ctx, id2, ownerID+"other", dur, someFunc)
	test.That(t, sess2.CheckOwnerID(ownerID), test.ShouldBeFalse)
	test.That(t, sess2.CheckOwnerID(ownerID+"other"), test.ShouldBeTrue)

	test.That(t, sess1.ID(), test.ShouldEqual, id1)
	test.That(t, sess2.ID(), test.ShouldEqual, id2)
	test.That(t, sess1.CheckOwnerID(ownerID), test.ShouldBeTrue)
	test.That(t, sess1.CheckOwnerID(ownerID+"other"), test.ShouldBeFalse)
	test.That(t, sess1.Active(now), test.ShouldBeTrue)
	time.Sleep(2 * dur)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeFalse)
	sess1.Heartbeat(ctx)
	test.That(t, sess1.Active(time.Now()), test.ShouldBeTrue)
	test.That(t, sess1.PeerConnectionInfo(), test.ShouldResembleProto, info)
	test.That(t, sess1.HeartbeatWindow(), test.ShouldEqual, dur)
	test.That(t, sess1.Deadline().After(now), test.ShouldBeTrue)
	test.That(t, sess1.Deadline().Before(now.Add(2*dur)), test.ShouldBeFalse)
}
