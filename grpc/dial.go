package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/utils/contextutils"
)

// Dial dials a gRPC server. `ctx` can be used to set a timeout/deadline for Dial.
// However, the max timeout is 20 seconds.
func Dial(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (rpc.ClientConn, error) {
	webrtcOpts := rpc.DialWebRTCOptions{
		Config: &DefaultWebRTCConfiguration,
	}

	if signalingServerAddress, secure, ok := InferSignalingServerAddress(address); ok {
		webrtcOpts.AllowAutoDetectAuthOptions = true
		webrtcOpts.SignalingInsecure = !secure
		webrtcOpts.SignalingServerAddress = signalingServerAddress
	}

	optsCopy := make([]rpc.DialOption, len(opts)+2)
	optsCopy[0] = rpc.WithWebRTCOptions(webrtcOpts)
	optsCopy[1] = rpc.WithAllowInsecureDowngrade()
	copy(optsCopy[2:], opts)

	ctx, cancel := contextutils.ContextWithTimeoutIfNoDeadline(ctx, 60*time.Second)
	defer cancel()

	return rpc.Dial(ctx, address, logger, optsCopy...)
}

// InferSignalingServerAddress returns the appropriate WebRTC signaling server address
// if it can be detected. Returns the address, if the endpoint is secure, and if found.
// TODO(RSDK-235):
// remove hard coding of signaling server address and
// prefer SRV lookup instead.
func InferSignalingServerAddress(address string) (string, bool, bool) {
	switch {
	case strings.HasSuffix(address, ".viam.cloud"):
		return "app.viam.com:443", true, true
	case strings.HasSuffix(address, ".robot.viaminternal"):
		return "app.viaminternal:8089", true, true
	case strings.HasSuffix(address, ".viamstg.cloud"):
		return "app.viam.dev:443", true, true
	}
	return "", false, false
}
