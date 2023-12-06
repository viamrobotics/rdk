package logging

import (
	"context"

	"go.viam.com/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type debugLogKeyType int

const debugLogKeyID = debugLogKeyType(iota)

// EnableDebugMode returns a new context with debug logging state attached with a randomly generated
// log key.
func EnableDebugMode(ctx context.Context) context.Context {
	return EnableDebugModeWithKey(ctx, "")
}

// EnableDebugModeWithKey returns a new context with debug logging state attached. An empty `debugLogKey`
// generates a random value.
func EnableDebugModeWithKey(ctx context.Context, debugLogKey string) context.Context {
	if debugLogKey == "" {
		// Allow enabling debug mode without an explicit `debugLogKey` will generate a random-ish
		// value such that the server logs can distinguish between multiple rpc operations.
		debugLogKey = utils.RandomAlphaString(6)
	}
	return context.WithValue(ctx, debugLogKeyID, debugLogKey)
}

// IsDebugMode returns whether the input context has debug logging enabled.
func IsDebugMode(ctx context.Context) bool {
	return GetName(ctx) != ""
}

// GetName returns the debug log key included when enabling the context for debug logging.
func GetName(ctx context.Context) string {
	valI := ctx.Value(debugLogKeyID)
	if val, ok := valI.(string); ok {
		return val
	}

	return ""
}

const dtNameMetadataKey = "dtName"

// UnaryClientInterceptor adds debug directives from the current context (if any) to the
// outgoing request's metadata.
func UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if IsDebugMode(ctx) {
		ctx = metadata.AppendToOutgoingContext(ctx, dtNameMetadataKey, GetName(ctx))
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

// UnaryServerInterceptor checks the incoming RPC metadata for a distributed tracing directive and
// attaches any information to a debug context.
func UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	values := meta.Get(dtNameMetadataKey)
	if len(values) == 1 {
		ctx = EnableDebugModeWithKey(ctx, values[0])
	}

	return handler(ctx, req)
}
