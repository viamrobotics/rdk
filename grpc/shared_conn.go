package grpc

import (
	"fmt"
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
	peerConn           *webrtc.PeerConnection
	logger             logging.Logger
	mu                 sync.RWMutex
	resourceOnTrackCBs map[resource.Name]OnTrackCB
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
			logger.Infow("OnTrack called with", "streamID", tr.StreamID())
			name, err := resource.NewFromString(tr.StreamID())
			if err != nil {
				logger.Errorw("OnTrack called with streamID which is not able to be parsed into a resource name", "streamID ", tr.StreamID())
				return
			}
			sc.mu.RLock()
			onTrackCB, ok := sc.resourceOnTrackCBs[name]
			sc.mu.RUnlock()
			if !ok {
				sc.logger.Errorw("OnTrack called with StreamID which is not in the resourceOnTrackCBs",
					"streamID", name, "resourceOnTrackCBs", fmt.Sprintf("%#v", sc.resourceOnTrackCBs))
				return
			}
			onTrackCB(tr, r)
		})
	}
	return sc
}

// AddOnTrackSub adds an OnTrack subscription for the resource.
func (sc *SharedConn) AddOnTrackSub(name resource.Name, onTrackCB OnTrackCB) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.resourceOnTrackCBs[name] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the resource.
func (sc *SharedConn) RemoveOnTrackSub(name resource.Name) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.resourceOnTrackCBs, name)
}

// PeerConn returns the PeerConnection.
func (sc *SharedConn) PeerConn() *webrtc.PeerConnection {
	return sc.peerConn
}

// Close closes a shared connection.
func (sc *SharedConn) Close() error {
	var err error
	if sc.peerConn != nil {
		err = sc.peerConn.Close()
	}

	return multierr.Combine(err, sc.ReconfigurableClientConn.Close())
}
