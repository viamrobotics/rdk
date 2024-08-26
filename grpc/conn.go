package grpc

import (
	"context"
	"errors"
	"sync"

	"github.com/viamrobotics/webrtc/v3"
	"go.viam.com/utils/rpc"
	googlegrpc "google.golang.org/grpc"
)

// ReconfigurableClientConn allows for the underlying client connections to be swapped under the hood.
type ReconfigurableClientConn struct {
	connMu sync.RWMutex
	conn   rpc.ClientConn
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
