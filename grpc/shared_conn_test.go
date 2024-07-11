package grpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/webrtc/v3"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
)

func TestNewLocalPeerConnection(t *testing.T) {
	t.Run("both NewLocalPeerConnections", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		client, err := NewLocalPeerConnection(logger)
		test.That(t, err, test.ShouldBeNil)
		server, err := NewLocalPeerConnection(logger)
		test.That(t, err, test.ShouldBeNil)
		var cMu sync.Mutex
		clientCandates := []*webrtc.ICECandidate{}
		client.OnICECandidate(func(i *webrtc.ICECandidate) {
			cMu.Lock()
			defer cMu.Unlock()
			clientCandates = append(clientCandates, i)
		})
		clientPeerConnReady, clientPeerConnClosed, err := rpc.ConfigureForRenegotiation(client, logger.AsZap())
		test.That(t, err, test.ShouldBeNil)

		var sMu sync.Mutex
		serverCandidates := []*webrtc.ICECandidate{}
		server.OnICECandidate(func(i *webrtc.ICECandidate) {
			sMu.Lock()
			defer sMu.Unlock()
			serverCandidates = append(serverCandidates, i)
		})

		defer func() {
			test.That(t, client.Close(), test.ShouldBeNil)
			<-clientPeerConnClosed
		}()
		serverPeerConnReady, serverPeerConnClosed, err := rpc.ConfigureForRenegotiation(server, logger.AsZap())
		test.That(t, err, test.ShouldBeNil)
		defer func() {
			server.Close()
			<-serverPeerConnClosed
		}()
		timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second*10)
		defer timeoutFn()

		signalPair(t, client, server)

		select {
		case <-timeoutCtx.Done():
			t.Log("timeout")
			t.FailNow()
		case <-clientPeerConnReady:
		}

		select {
		case <-timeoutCtx.Done():
			t.Log("timeout")
			t.FailNow()
		case <-serverPeerConnReady:
		}

		// proves that only the loopback address is considered an ice candidate for both
		// client and server
		cMu.Lock()
		test.That(t, len(clientCandates), test.ShouldEqual, 2)
		test.That(t, len(clientCandates), test.ShouldEqual, 2)
		test.That(t, clientCandates[1], test.ShouldBeNil)
		test.That(t, clientCandates[0], test.ShouldNotBeNil)
		test.That(t, clientCandates[0].Address, test.ShouldResemble, "127.0.0.1")
		cMu.Unlock()

		sMu.Lock()
		test.That(t, len(serverCandidates), test.ShouldEqual, 2)
		test.That(t, serverCandidates[1], test.ShouldBeNil)
		test.That(t, serverCandidates[0], test.ShouldNotBeNil)
		test.That(t, serverCandidates[0].Address, test.ShouldResemble, "127.0.0.1")
		sMu.Unlock()
	})

	t.Run("one NewLocalPeerConnection non local server", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		client, err := NewLocalPeerConnection(logger)
		test.That(t, err, test.ShouldBeNil)
		server, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		test.That(t, err, test.ShouldBeNil)

		testClientServer(t, client, server, logger)
	})

	t.Run("one NewLocalPeerConnection non local client", func(t *testing.T) {
		logger := logging.NewTestLogger(t)
		server, err := NewLocalPeerConnection(logger)
		test.That(t, err, test.ShouldBeNil)
		client, err := webrtc.NewPeerConnection(webrtc.Configuration{})
		test.That(t, err, test.ShouldBeNil)

		testClientServer(t, client, server, logger)
	})
}

func signalPair(t *testing.T, left, right *webrtc.PeerConnection) {
	t.Helper()

	leftOffer, err := left.CreateOffer(nil)
	test.That(t, err, test.ShouldBeNil)
	err = left.SetLocalDescription(leftOffer)
	test.That(t, err, test.ShouldBeNil)
	<-webrtc.GatheringCompletePromise(left)

	leftOffer.SDP = left.LocalDescription().SDP
	test.That(t, right.SetRemoteDescription(leftOffer), test.ShouldBeNil)

	rightAnswer, err := right.CreateAnswer(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, right.SetLocalDescription(rightAnswer), test.ShouldBeNil)
	<-webrtc.GatheringCompletePromise(right)
	test.That(t, left.SetRemoteDescription(rightAnswer), test.ShouldBeNil)
}

func testClientServer(t *testing.T, client, server *webrtc.PeerConnection, logger logging.Logger) {
	clientPeerConnReady, clientPeerConnClosed, err := rpc.ConfigureForRenegotiation(client, logger.AsZap())
	test.That(t, err, test.ShouldBeNil)

	defer func() {
		test.That(t, client.Close(), test.ShouldBeNil)
		<-clientPeerConnClosed
	}()
	serverPeerConnReady, serverPeerConnClosed, err := rpc.ConfigureForRenegotiation(server, logger.AsZap())
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		server.Close()
		<-serverPeerConnClosed
	}()
	timeoutCtx, timeoutFn := context.WithTimeout(context.Background(), time.Second*10)
	defer timeoutFn()

	signalPair(t, client, server)

	select {
	case <-timeoutCtx.Done():
		t.Log("timeout")
		t.FailNow()
	case <-clientPeerConnReady:
	}

	select {
	case <-timeoutCtx.Done():
		t.Log("timeout")
		t.FailNow()
	case <-serverPeerConnReady:
	}
}
