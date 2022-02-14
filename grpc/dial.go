package grpc

import (
	"context"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
)

// Dial dials a gRPC server.
func Dial(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (rpc.ClientConn, error) {
	optsCopy := make([]rpc.DialOption, len(opts)+2)
	optsCopy[0] = rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
		Config: &DefaultWebRTCConfiguration,
	})
	optsCopy[1] = rpc.WithAllowInsecureDowngrade()
	copy(optsCopy[2:], opts)

	ctx, timeoutCancel := context.WithTimeout(ctx, 20*time.Second)
	defer timeoutCancel()

	return rpc.Dial(ctx, address, logger, optsCopy...)
}
