package testutils

import (
	"context"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
)

// TrackingDialer tracks dial attempts.
type TrackingDialer struct {
	rpc.Dialer
	DialCalled int
}

// DialDirect tracks calls of DialDirect.
func (td *TrackingDialer) DialDirect(
	ctx context.Context,
	target string,
	onClose func() error,
	opts ...grpc.DialOption,
) (rpc.ClientConn, bool, error) {
	td.DialCalled++
	return td.Dialer.DialDirect(ctx, target, onClose, opts...)
}

// DialFunc tracks calls of DialFunc.
func (td *TrackingDialer) DialFunc(
	proto string,
	target string,
	f func() (rpc.ClientConn, func() error, error),
) (rpc.ClientConn, bool, error) {
	td.DialCalled++
	return td.Dialer.DialFunc(proto, target, f)
}
