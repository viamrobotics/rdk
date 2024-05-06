package grpc

import (
	"context"
	"errors"
	"sync"

	"github.com/pion/webrtc/v3"
	"go.viam.com/rdk/resource"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
)

// ReconfigurableClientConn allows for the underlying client connections to be swapped under the hood.
type ReconfigurableClientConn struct {
	connMu sync.RWMutex
	conn   rpc.ClientConn

	resOnTrackMu  sync.Mutex
	resOnTrackCBs map[resource.Name]OnTrackCB
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
	if c.resOnTrackCBs == nil {
		c.resOnTrackCBs = make(map[resource.Name]OnTrackCB)
	}

	if pc := conn.PeerConn(); pc != nil {
		pc.OnTrack(func(trackRemote *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
			name, err := resource.NewFromString(trackRemote.StreamID())
			if err != nil {
				// sc.logger.Errorw("StreamID did not parse as a ResourceName", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
				return
			}
			c.resOnTrackMu.Lock()
			onTrackCB, ok := c.resOnTrackCBs[name]
			c.resOnTrackMu.Unlock()
			if !ok {
				// sc.logger.Errorw("Callback not found for StreamID", "sharedConn", fmt.Sprintf("%p", sc), "streamID", trackRemote.StreamID())
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

// AddOnTrackSub adds an OnTrack subscription for the resource.
func (c *ReconfigurableClientConn) AddOnTrackSub(name resource.Name, onTrackCB OnTrackCB) {
	c.resOnTrackMu.Lock()
	defer c.resOnTrackMu.Unlock()
	c.resOnTrackCBs[name] = onTrackCB
}

// RemoveOnTrackSub removes an OnTrack subscription for the resource.
func (c *ReconfigurableClientConn) RemoveOnTrackSub(name resource.Name) {
	c.resOnTrackMu.Lock()
	defer c.resOnTrackMu.Unlock()
	delete(c.resOnTrackCBs, name)
}
