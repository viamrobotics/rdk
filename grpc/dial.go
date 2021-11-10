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

// Info stores info for a grpc client that can be used to create another grpc client
type Info struct {
	Address     string
	DialOptions rpcclient.DialOptions

	Logger golog.Logger
}

func (d Info) DialInfo() Info {
	return d
}

// DialInfoGetter defines a method to get DialInfo
type DialInfoGetter interface {
	DialInfo() Info
}
