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
type SharedConn struct {
	grpcConn      ReconfigurableClientConn
	peerConn      *webrtc.PeerConnection
	PeerConnReady <-chan struct{}

	resOnTrackMu  sync.Mutex
	resOnTrackCBs map[resource.Name]OnTrackCB
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
func (sc *SharedConn) ResetConn(conn rpc.ClientConn) {
	sc.grpcConn.ReplaceConn(conn)

	logger := logging.Global().Sublogger("SharedConn")

	peerConn, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		logger.Warnw("Unable to create optional peer connection for module. Ignoring.", "err", err)
		return
	}

	sc.peerConn = peerConn
	sc.PeerConnReady, err = rpc.ConfigureForRenegotiation(peerConn, logger.AsZap())
	if err != nil {
		logger.Warnw("Unable to create optional renegotiation channel for module. Ignoring.", "err", err)
		utils.UncheckedError(sc.peerConn.Close())
		sc.peerConn = nil
		return
	}

	sc.peerConn.OnTrack(func(trackRemote *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		name, err := resource.NewFromString(trackRemote.StreamID())
		if err != nil {
			logger.Errorw("StreamID did not parse as a ResourceName", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
			return
		}
		sc.resOnTrackMu.Lock()
		onTrackCB, ok := sc.resOnTrackCBs[name]
		sc.resOnTrackMu.Unlock()
		if !ok {
			logger.Errorw("Callback not found for StreamID", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
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
