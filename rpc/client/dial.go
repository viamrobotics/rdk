// Package client provides a multi-faceted approach for connecting to a server.
package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/edaniels/golog"
	"google.golang.org/grpc"

	"go.viam.com/core/rpc/dialer"
	rpcwebrtc "go.viam.com/core/rpc/webrtc"
)

// DialOptions are extra dial time options.
type DialOptions struct {
	// Secure determines if the RPC connection is TLS based.
	Secure bool
}

// Dial attempts to make the most convenient connection to the given address. It first tries a direct
// connection if the address is an IP. It next tries to connect to the local version of the host followed
// by a WebRTC brokered connection.
func Dial(ctx context.Context, address string, opts DialOptions, logger golog.Logger) (dialer.ClientConn, error) {
	var host string
	var port string
	if strings.ContainsRune(address, ':') {
		var err error
		host, port, err = net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
	}

	if addr := net.ParseIP(host); addr == nil {
		localHost := fmt.Sprintf("local.%s", host)
		if _, err := net.LookupHost(localHost); err == nil {
			localAddress := localHost
			if port != "" {
				localAddress = fmt.Sprintf("%s:%s", localHost, port)
			}
			// TODO(erd): This needs to authenticate the server so we don't have a confused
			// deputy.
			if conn, err := dialDirectGRPC(ctx, localAddress, opts); err == nil {
				logger.Debugw("connected directly via local host", "address", localAddress)
				return conn, nil
			}
		}
	}

	conn, err := rpcwebrtc.Dial(ctx, address, logger)
	if err != nil {
		if errors.Is(err, rpcwebrtc.ErrNoSignaler) {
			if conn, err := dialDirectGRPC(ctx, address, opts); err == nil {
				logger.Debugw("connected directly", "address", address)
				return conn, nil
			}
		}
		return nil, err
	}
	logger.Debug("connected via WebRTC")
	return conn, nil
}

func dialDirectGRPC(ctx context.Context, address string, opts DialOptions) (dialer.ClientConn, error) {
	dialOpts := []grpc.DialOption{grpc.WithBlock(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1 << 24))}
	// if this is secure, there's no way via DialOptions to set credentials yet
	if !opts.Secure {
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}
	if ctxDialer := dialer.ContextDialer(ctx); ctxDialer != nil {
		return ctxDialer.Dial(ctx, address, dialOpts...)
	}
	return grpc.DialContext(ctx, address, dialOpts...)
}
