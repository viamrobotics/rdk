package grpc

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	rpcclient "go.viam.com/utils/rpc/client"
	"go.viam.com/utils/rpc/dialer"
)

// Dial dials a gRPC server.
func Dial(ctx context.Context, address string, opts rpcclient.DialOptions, logger golog.Logger) (dialer.ClientConn, error) {
	ctx, timeoutCancel := context.WithTimeout(ctx, 20*time.Second)
	defer timeoutCancel()

	if opts.WebRTC.Config == nil {
		opts.WebRTC.Config = &DefaultWebRTCConfiguration
	}
	return rpcclient.Dial(ctx, address, opts, logger)
}
