package grpc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"go.uber.org/multierr"
	rutils "go.viam.com/rdk/utils"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// OnTrackCB is the signature of the OnTrack callback function
// a resource may register with a SharedConn which supports WebRTC.
type OnTrackCB func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver)

// SharedConn wraps both a GRPC connection & (optionally) a peer connection & controls access to both.
// For modules, the grpc connection is over a Unix socket. The WebRTC `PeerConnection` is made
// separately. The `SharedConn` continues to implement the `rpc.ClientConn` interface by pairing up
// the two underlying connections a client may want to communicate over.
//
// The lifetime of SharedConn objects are a bit nuanced. SharedConn objects are expected to be
// handed to resource Client objects. When connections break and become reestablished, those client
// objects are not recreated, but rather we swap in new connection objects under the hood.
//
// This is because resource management controls updating the graph nodes with Client objects only
// when calls to `Reconfigure` are made. But connections can break and heal without any calls to
// Reconfigure.
//
// Thus the lifetime of a SharedConn for a client is _not_:
// 1) Object initialize
// 2) Working
// 3) Close
// 4) Object destruction
// And back to initialization.
//
// But rather:
// 1) Object initialization
// 2) `ResetConn`
// 3) `Close`
// 4) `ResetConn`
// ...
// 5) Object destruction (at shutdown or resource removed from config).
//
// Each call to `ResetConn` is a new "generation" of connections.
type SharedConn struct {
	grpcConn ReconfigurableClientConn

	// `peerConnMu` synchronizes changes to the underlying `peerConn`. Such that calls consecutive
	// calls to `GrpcConn` and `PeerConn` will return connections from the same (or newer, but not
	// prior) "generations".
	peerConnMu     sync.Mutex
	peerConn       *webrtc.PeerConnection
	peerConnReady  <-chan struct{}
	peerConnFailed chan struct{}

	resOnTrackMu  sync.Mutex
	resOnTrackCBs map[resource.Name]OnTrackCB

	logger logging.Logger
}

// Invoke forwards to the underlying GRPC Connection.
func (sc *SharedConn) Invoke(
	ctx context.Context,
	method string,
	args, reply interface{},
	opts ...googlegrpc.CallOption,
) error {
	return sc.grpcConn.Invoke(ctx, method, args, reply, opts...)
}

// NewStream forwards to the underlying GRPC Connection.
func (sc *SharedConn) NewStream(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	method string,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	return sc.grpcConn.NewStream(ctx, desc, method, opts...)
}

// AddOnTrackSub adds an OnTrack subscription for the resource.
func (sc *SharedConn) AddOnTrackSub(name resource.Name, onTrackCB OnTrackCB) {
	sc.resOnTrackMu.Lock()
	defer sc.resOnTrackMu.Unlock()
	sc.resOnTrackCBs[name] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the resource.
func (sc *SharedConn) RemoveOnTrackSub(name resource.Name) {
	sc.resOnTrackMu.Lock()
	defer sc.resOnTrackMu.Unlock()
	delete(sc.resOnTrackCBs, name)
}

// GrpcConn returns a gRPC capable client connection.
func (sc *SharedConn) GrpcConn() *ReconfigurableClientConn {
	return &sc.grpcConn
}

// PeerConn returns a WebRTC PeerConnection capable of sending video/audio data.
func (sc *SharedConn) PeerConn() *webrtc.PeerConnection {
	// Block until the peer connection result is known.
	select {
	case <-sc.peerConnReady:
	case <-sc.peerConnFailed:
	}

	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()
	return sc.peerConn
}

// ResetConn acts as a constructor for `SharedConn`. ResetConn replaces the underlying
// connection objects in addition to some other initialization.
//
// The first call to `ResetConn` is guaranteed to happen before any access to connection objects
// happens. But subequent calls can be entirely asynchronous to components/services accessing
// `SharedConn` for connection objects.
func (sc *SharedConn) ResetConn(conn rpc.ClientConn, moduleLogger logging.Logger) {
	sc.grpcConn.ReplaceConn(conn)
	if sc.logger == nil {
		// The first call to `ResetConn` happens before anything can access `sc.logger`. So long as
		// we never write to the member variable, everything can continue to access this without
		// locks.
		sc.logger = moduleLogger.Sublogger("conn")
	}

	if sc.resOnTrackCBs == nil {
		// Same initilization argument as above with the logger.
		sc.resOnTrackCBs = make(map[resource.Name]OnTrackCB)
	}

	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()

	if sc.peerConn != nil {
		sc.logger.Warn("SharedConn is being reset with an active peer connection.")
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
	}

	peerConn, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		sc.logger.Warnw("Unable to create optional peer connection for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn = peerConn
	sc.peerConnFailed = make(chan struct{})
	// If initializing a PeerConnection fails for any reason, we perform the following cleanup
	// steps.

	guard := rutils.NewGuard(func() {
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
		close(sc.peerConnFailed)
	})
	defer guard.OnFail()

	sc.peerConnReady, err = rpc.ConfigureForRenegotiation(peerConn, sc.logger.AsZap())
	if err != nil {
		sc.logger.Warnw("Unable to create optional renegotiation channel for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn.OnTrack(func(trackRemote *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		name, err := resource.NewFromString(trackRemote.StreamID())
		if err != nil {
			sc.logger.Errorw("StreamID did not parse as a ResourceName", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
			return
		}
		sc.resOnTrackMu.Lock()
		onTrackCB, ok := sc.resOnTrackCBs[name]
		sc.resOnTrackMu.Unlock()
		if !ok {
			sc.logger.Errorw("Callback not found for StreamID", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
			return
		}
		onTrackCB(trackRemote, rtpReceiver)
	})

	guard.Success()
}

// GenerateEncodedOffer creates a WebRTC offer that's JSON + base64 encoded. If an error is
// returned, `SharedConn.PeerConn` will return nil until a following `Reset`.
func (sc *SharedConn) GenerateEncodedOffer() (string, error) {
	// If this generating an offer fails for any reason, we perform the following cleanup steps.
	guard := rutils.NewGuard(func() {
		sc.peerConnMu.Lock()
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
		close(sc.peerConnFailed)
		sc.peerConnMu.Unlock()
	})
	defer guard.OnFail()

	pc := sc.peerConn
	if pc == nil {
		return "", errors.New("peer connections disabled")
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-webrtc.GatheringCompletePromise(pc)
	ret, err := rpc.EncodeSDP(pc.LocalDescription())
	if err != nil {
		return "", err
	}

	guard.Success()
	return ret, nil
}

// ProcessEncodedAnswer sets the remote description to the answer and waits for the connection to be
// ready as per the negotiation channel being opened. The answer is expected to be JSON + base64
// encoded. If an error is returned, `SharedConn.PeerConn` will return nil until a following
// `Reset`.
func (sc *SharedConn) ProcessEncodedAnswer(encodedAnswer string) error {
	guard := rutils.NewGuard(func() {
		sc.peerConnMu.Lock()
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
		close(sc.peerConnFailed)
		sc.peerConnMu.Unlock()
	})
	defer guard.OnFail()

	pc := sc.peerConn
	if pc == nil {
		return errors.New("PeerConnection was not initialized")
	}

	answer := webrtc.SessionDescription{}
	if err := rpc.DecodeSDP(encodedAnswer, &answer); err != nil {
		return err
	}

	if err := pc.SetRemoteDescription(answer); err != nil {
		return err
	}

	guard.Success()
	return nil
}

// Close closes a shared connection.
func (sc *SharedConn) Close() error {
	var err error
	sc.peerConnMu.Lock()
	if sc.peerConn != nil {
		err = sc.peerConn.Close()
		sc.peerConn = nil
		close(sc.peerConnFailed)
	}
	sc.peerConnMu.Unlock()

	return multierr.Combine(err, sc.grpcConn.Close())
}
