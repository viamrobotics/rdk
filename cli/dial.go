package cli

import (
	"context"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// appKeepAliveParams are the client keepalive parameters used for connections to app. goutils'
// dialer sets PermitWithoutStream, which keeps gRPC pinging a connection that has no RPCs in
// flight. The CLI's connections to app are idle by design -- one is held for the life of the
// process -- and app's ingress answers those idle pings with a GOAWAY (ENHANCE_YOUR_CALM,
// "too_many_pings") that tears the connection down. There is no safe idle ping interval to
// reconcile with instead: a gRPC server counts any idle ping as a violation unless hours have
// passed since the previous one. Pinging only while an RPC is in flight still detects a dead
// connection when it matters.
var appKeepAliveParams = keepalive.ClientParameters{
	Time:                rpc.KeepAliveTime * 2, // keep this in sync with goutils' rpc/dialer.
	PermitWithoutStream: false,
}

// appDialer dials connections to appHost with appKeepAliveParams and leaves every other
// connection, a machine connection for example, on goutils' parameters. Connection caching and
// WebRTC dialing are goutils' as well.
type appDialer struct {
	rpc.Dialer
	appHost string
}

// newAppDialer returns a dialer to attach to a dial context with rpc.ContextWithDialer. Closing
// it closes the connections it dialed.
func newAppDialer(appHost string) rpc.Dialer {
	return &appDialer{Dialer: rpc.NewCachedDialer(), appHost: appHost}
}

// DialDirect dials a direct gRPC connection, overriding the keepalive parameters for connections
// to app. gRPC applies dial options in order, so appending ours wins over goutils'.
func (d *appDialer) DialDirect(
	ctx context.Context,
	target string,
	keyExtra string,
	onClose func() error,
	opts ...grpc.DialOption,
) (rpc.ClientConn, bool, error) {
	if target == d.appHost {
		opts = append(opts, grpc.WithKeepaliveParams(appKeepAliveParams))
	}
	return d.Dialer.DialDirect(ctx, target, keyExtra, onClose, opts...)
}
