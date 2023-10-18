package grpc

import (
	"context"
	"strings"
	"time"

	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils/contextutils"
)

// defaultDialTimeout is the default timeout for dialing a robot.
var defaultDialTimeout = 20 * time.Second

// Dial dials a gRPC server. `ctx` can be used to set a timeout/deadline for Dial. However, the signaling
// server may have other timeouts which may prevent the full timeout from being respected.
func Dial(ctx context.Context, address string, logger logging.Logger, opts ...rpc.DialOption) (rpc.ClientConn, error) {
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

	ctx, cancel := contextutils.ContextWithTimeoutIfNoDeadline(ctx, defaultDialTimeout)
	defer cancel()

	return rpc.Dial(ctx, address, logger.AsZap(), optsCopy...)
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
