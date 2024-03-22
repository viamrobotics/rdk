package grpc

import (
	"sync"

	"github.com/pion/webrtc/v3"
	"go.uber.org/multierr"

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
	*ReconfigurableClientConn
	peerConnMu         sync.RWMutex
	resourceOnTrackCBs map[resource.Name]OnTrackCB
	peerConn           *webrtc.PeerConnection
	logger             logging.Logger
}

// NewSharedConn returns a SharedConnection which manages access to both a grpc client conntection & (optionally) a
// webrtc peer connection.
func NewSharedConn(conn *ReconfigurableClientConn, peerConn *webrtc.PeerConnection, logger logging.Logger) *SharedConn {
	sc := &SharedConn{
		ReconfigurableClientConn: conn,
		peerConn:                 peerConn,
		resourceOnTrackCBs:       map[resource.Name]OnTrackCB{},
		logger:                   logger,
	}
	if peerConn != nil {
		peerConn.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
			name, err := resource.NewFromString(tr.StreamID())
			if err != nil {
				logger.Errorf("%p OnTrack called with StreamID: %s which is not able to be parsed into a resource name", sc, tr.StreamID())
				return
			}
			// TODO: Maybe we don't want to hold this lock for the entire duration of the onTrackCB
			// check this later
			sc.peerConnMu.RLock()
			defer sc.peerConnMu.RUnlock()
			onTrackCB, ok := sc.resourceOnTrackCBs[name]
			if !ok {
				sc.logger.Errorf("%p OnTrack called with StreamID: %s which is not in the resourceOnTrackCBs: %#v", sc, name, sc.resourceOnTrackCBs)
				return
			}
			onTrackCB(tr, r)
		})
	}
	return sc
}

// AddOnTrackSub adds an OnTrack subscription for the resource.
func (sc *SharedConn) AddOnTrackSub(name resource.Name, onTrackCB OnTrackCB) {
	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()
	sc.resourceOnTrackCBs[name] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the resource.
func (sc *SharedConn) RemoveOnTrackSub(name resource.Name) {
	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()
	delete(sc.resourceOnTrackCBs, name)
}

// PeerConn returns the PeerConnection.
func (sc *SharedConn) PeerConn() *webrtc.PeerConnection {
	sc.peerConnMu.Lock()
	defer sc.peerConnMu.Unlock()
	return sc.peerConn
}

// Close closes a shared connection.
func (sc *SharedConn) Close() error {
	var err error
	sc.peerConnMu.Lock()
	if sc.peerConn != nil {
		err = sc.peerConn.Close()
	}
	sc.peerConnMu.Unlock()

	return multierr.Combine(err, sc.ReconfigurableClientConn.Close())
}
