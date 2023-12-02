package logging

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type debugModeType int
type debugLogKeyType int

const debugModeID = debugModeType(iota)
const debugLogKeyID = debugLogKeyType(iota)

func EnableDebugMode(ctx context.Context, debugLogKey string) context.Context {
	ctx = context.WithValue(ctx, debugModeID, true)
	return context.WithValue(ctx, debugLogKeyID, debugLogKey)
}

func IsDebugMode(ctx context.Context) bool {
	isDebug, ok := ctx.Value(debugModeID).(bool)
	if !ok {
		return false
	}
	return isDebug
}

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

// UnaryServerInterceptor creates a new operation in the current context before passing
// it to the unary response handler. If the incoming RPC metadata contains a directive for
// distributed tracing, attach that to the context.
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
		ctx = EnableDebugMode(ctx, values[0])
	}

	return handler(ctx, req)
}
