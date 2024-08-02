package grpc

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"

	"go.viam.com/rdk/config"
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

	opts = append(
		opts, 
		rpc.WithUnaryClientInterceptor(unaryClientInterceptor()), 
		rpc.WithStreamClientInterceptor(streamClientInterceptor()),
	)

	optsCopy := make([]rpc.DialOption, len(opts)+2)
	optsCopy[0] = rpc.WithWebRTCOptions(webrtcOpts)
	optsCopy[1] = rpc.WithAllowInsecureDowngrade()
	copy(optsCopy[2:], opts)

	ctx, cancel := contextutils.ContextWithTimeoutIfNoDeadline(ctx, defaultDialTimeout)
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

func addVersionMetadataToContext(ctx context.Context) context.Context {
	version := config.Version
	if version == "" {
		version = "dev-"
		if config.GitRevision != "" {
			version = config.GitRevision
		} else {
			version = "unknown"
		}
	}
	info, _ := debug.ReadBuildInfo()
	deps := make(map[string]*debug.Module, len(info.Deps))
	for _, dep := range info.Deps {
			deps[dep.Path] = dep
	}
	apiVersion := "?"
	if dep, ok := deps["go.viam.com/api"]; ok {
			apiVersion = dep.Version
	}
	versionMetadata := fmt.Sprintf("go;v%s;v%s", version, apiVersion)
	return context.WithValue(ctx, "viam_client", versionMetadata)
}

func unaryClientInterceptor() grpc.UnaryClientInterceptor{
	return func(
			ctx context.Context,
			method string,
			req, reply interface{},
			cc *grpc.ClientConn,
			invoker grpc.UnaryInvoker,
			opts ...grpc.CallOption,
	) error {
		ctx = addVersionMetadataToContext(ctx)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func streamClientInterceptor() grpc.StreamClientInterceptor {
	return func(
		ctx context.Context, 
		desc *grpc.StreamDesc, 
		cc *grpc.ClientConn, 
		method string, 
		streamer grpc.Streamer, 
		opts ...grpc.CallOption,
	) (cs grpc.ClientStream, err error) {
		ctx = addVersionMetadataToContext(ctx)
		return streamer(ctx, desc, cc, method, opts...)
	}
}
