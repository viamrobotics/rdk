// Package testutils implements test utilities.
package testutils

import (
	"context"
	"sync"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// TrackingDialer tracks dial attempts.
type TrackingDialer struct {
	rpc.Dialer
	NewConnections int
}

// DialDirect tracks calls of DialDirect.
func (td *TrackingDialer) DialDirect(
	ctx context.Context,
	target string,
	keyExtra string,
	onClose func() error,
	opts ...grpc.DialOption,
) (rpc.ClientConn, bool, error) {
	conn, cached, err := td.Dialer.DialDirect(ctx, target, keyExtra, onClose, opts...)
	if err != nil {
		return nil, false, err
	}
	if !cached {
		td.NewConnections++
	}
	return conn, cached, err
}

// DialFunc tracks calls of DialFunc.
func (td *TrackingDialer) DialFunc(
	proto string,
	target string,
	keyExtra string,
	f func() (rpc.ClientConn, func() error, error),
) (rpc.ClientConn, bool, error) {
	conn, cached, err := td.Dialer.DialFunc(proto, target, keyExtra, f)
	if err != nil {
		return nil, false, err
	}
	if !cached {
		td.NewConnections++
	}
	return conn, cached, err
}

type ServerTransportStream struct {
	mu sync.Mutex
	grpc.ServerTransportStream
	md metadata.MD
}

func NewServerTransportStream() *ServerTransportStream {
	return &ServerTransportStream{}
}

func (s *ServerTransportStream) SetHeader(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.md = md.Copy()
	return nil
}

func (s *ServerTransportStream) SendHeader(md metadata.MD) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.md = md.Copy()
	return nil
}

func (s *ServerTransportStream) Value(key string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.md[key]
}
