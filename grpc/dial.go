package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
)

// Dial dials a gRPC server.
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

	ctx, timeoutCancel := context.WithTimeout(ctx, 20*time.Second)
	defer timeoutCancel()

	return rpc.Dial(ctx, address, logger, optsCopy...)
}

// InferSignalingServerAddress returns the appropriate WebRTC signaling server address
// if it can be detected.
// TODO(RSDK-235):
// remove hard coding of signaling server address and
// prefer SRV lookup instead.
func InferSignalingServerAddress(address string) (string, bool, bool) {
	switch {
	case strings.HasSuffix(address, ".viam.cloud"):
		return "app.viam.com:443", true, true
	case strings.HasSuffix(address, ".robot.viaminternal"):
		return "app.viaminternal:8089", false, true
	}
	return "", false, false
}
