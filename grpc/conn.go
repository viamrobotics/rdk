package grpc

import (
	"context"
	"errors"
	"sync"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/maps"
	googlegrpc "google.golang.org/grpc"

	"go.viam.com/rdk/logging"
)

// ReconfigurableClientConn allows for the underlying client connections to be swapped under the
// hood. A ReconfigurableClientConn may only be used for a connection to a single logical server.
type ReconfigurableClientConn struct {
	connMu sync.RWMutex
	conn   rpc.ClientConn

	onTrackCBByTrackNameMu sync.Mutex
	onTrackCBByTrackName   map[string]OnTrackCB
}

// Return this constant such that backoff error logging can compare consecutive errors and reliably
// conclude they are the same.
var errNotConnected = errors.New("not connected")

// Invoke invokes using the underlying client connection. In the case of c.conn being closed in the middle of
// an Invoke call, it is expected that c.conn can handle that and return a well-formed error.
func (c *ReconfigurableClientConn) Invoke(
	ctx context.Context,
	method string,
	args, reply interface{},
	opts ...googlegrpc.CallOption,
) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return errNotConnected
	}
	return conn.Invoke(ctx, method, args, reply, opts...)
}

// NewStream creates a new stream using the underlying client connection. In the case of c.conn being closed in the middle of
// a NewStream call, it is expected that c.conn can handle that and return a well-formed error.
func (c *ReconfigurableClientConn) NewStream(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	method string,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()
	if conn == nil {
		return nil, errNotConnected
	}
	return conn.NewStream(ctx, desc, method, opts...)
}

// ReplaceConn replaces the underlying client connection with the connection passed in. This does not close the
// old connection, the caller is expected to close it if needed.
func (c *ReconfigurableClientConn) ReplaceConn(conn rpc.ClientConn) {
	c.connMu.Lock()
	c.conn = conn
	// It is safe to access this without a mutex as it is only ever nil once at the beginning of the
	// ReconfigurableClientConn's lifetime. Before it is shared with clients.
	if c.onTrackCBByTrackName == nil {
		c.onTrackCBByTrackName = make(map[string]OnTrackCB)
	}

	if pc := conn.PeerConn(); pc != nil {
		pc.OnTrack(func(trackRemote *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
			c.onTrackCBByTrackNameMu.Lock()
			onTrackCB, ok := c.onTrackCBByTrackName[trackRemote.StreamID()]
			c.onTrackCBByTrackNameMu.Unlock()
			if !ok {
				logging.Global().Errorf("Callback not found for StreamID (trackName): %s, keys(resOnTrackCBs): %#v",
					trackRemote.StreamID(), maps.Keys(c.onTrackCBByTrackName))
				return
			}
			onTrackCB(trackRemote, rtpReceiver)
		})
	}
	c.connMu.Unlock()
}

// PeerConn returns the backing PeerConnection object, if applicable. Nil otherwise.
func (c *ReconfigurableClientConn) PeerConn() *webrtc.PeerConnection {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return nil
	}

	return c.conn.PeerConn()
}

// Close attempts to close the underlying client connection if there is one.
func (c *ReconfigurableClientConn) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn == nil {
		return nil
	}
	conn := c.conn
	c.conn = nil
	return conn.Close()
}

// AddOnTrackSub adds an OnTrack subscription for the track.
func (c *ReconfigurableClientConn) AddOnTrackSub(trackName string, onTrackCB OnTrackCB) {
	c.onTrackCBByTrackNameMu.Lock()
	defer c.onTrackCBByTrackNameMu.Unlock()
	c.onTrackCBByTrackName[trackName] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the track.
func (c *ReconfigurableClientConn) RemoveOnTrackSub(trackName string) {
	c.onTrackCBByTrackNameMu.Lock()
	defer c.onTrackCBByTrackNameMu.Unlock()
	delete(c.onTrackCBByTrackName, trackName)
}
