package grpc

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pion/webrtc/v3"
	"go.uber.org/multierr"
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
// 2) `ReplaceConn`
// 3) `Close`
// 4) `ReplaceConn`
// ...
// 5) Object destruction (at shutdown or resource removed from config)
type SharedConn struct {
	grpcConn      ReconfigurableClientConn
	peerConn      *webrtc.PeerConnection
	PeerConnReady <-chan struct{}

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
	return sc.peerConn
}

// ResetConn replaces the underlying GrpcConnection object. It also re-initiatlizes the
// PeerConnection that must be renegotiated.
func (sc *SharedConn) ResetConn(conn rpc.ClientConn, moduleLogger logging.Logger) {
	sc.grpcConn.ReplaceConn(conn)

	sc.logger = moduleLogger.Sublogger("conn")

	peerConn, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		sc.logger.Warnw("Unable to create optional peer connection for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn = peerConn
	sc.PeerConnReady, err = rpc.ConfigureForRenegotiation(peerConn, sc.logger.AsZap())
	if err != nil {
		sc.logger.Warnw("Unable to create optional renegotiation channel for module. Ignoring.", "err", err)
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
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
}

// GenerateEncodedOffer creates a WebRTC offer that's JSON + base64 encoded. If an error is
// returned, `SharedConn.PeerConn` will return nil until a following `Reset`.
func (sc *SharedConn) GenerateEncodedOffer() (string, error) {
	success := false
	defer func() {
		// Only keep the `peerConn` pointer active if we believe the connection is useable.
		if !success {
			sc.peerConn = nil
		}
	}()

	pc := sc.PeerConn()
	if pc == nil {
		return "", nil
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-webrtc.GatheringCompletePromise(pc)
	success = true
	return rpc.EncodeSDP(pc.LocalDescription())
}

// ProcessEncodedAnswer sets the remote description to the answer. The answer is expected to be JSON
// + base64 encoded. If an error is returned, `SharedConn.PeerConn` will return nil until a
// following `Reset`.
func (sc *SharedConn) ProcessEncodedAnswer(encodedAnswer string) error {
	success := false
	defer func() {
		// Only keep the `peerConn` pointer active if we believe the connection is useable.
		if !success {
			sc.peerConn = nil
		}
	}()

	pc := sc.PeerConn()
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

	<-sc.PeerConnReady
	success = true
	return nil
}

// Close closes a shared connection.
func (sc *SharedConn) Close() error {
	var err error
	if sc.peerConn != nil {
		err = sc.peerConn.Close()
	}

	return multierr.Combine(err, sc.grpcConn.Close())
}
