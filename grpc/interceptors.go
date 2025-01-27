package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// DefaultMethodTimeout is the default context timeout for all inbound gRPC
// methods and all outbound gRPC methods to modules, only used when no
// deadline is set on the context.
var DefaultMethodTimeout = 10 * time.Minute

// EnsureTimeoutUnaryServerInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary server interceptor.
func EnsureTimeoutUnaryServerInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (interface{}, error) {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultMethodTimeout)
		defer cancel()
	}

	return handler(ctx, req)
}

// EnsureTimeoutUnaryClientInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary client interceptor.
func EnsureTimeoutUnaryClientInterceptor(
	ctx context.Context,
	method string, req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, DefaultMethodTimeout)
		defer cancel()
	}

	return invoker(ctx, method, req, reply, cc, opts...)
}

// The following code is for appending/extracting grpc metadata regarding module names/origins via
// contexts.
type modNameKeyType int

const modNameKeyID = modNameKeyType(iota)

// GetName returns the debug log key included when enabling the context for debug logging.
func GetModuleName(ctx context.Context) string {
	valI := ctx.Value(modNameKeyID)
	if val, ok := valI.(string); ok {
		return val
	}

	return ""
}

const modNameMetadataKey = "modName"

type ModInterceptors struct {
	ModName string
}

// UnaryClientInterceptor adds debug directives from the current context (if any) to the
// outgoing request's metadata.
func (mc *ModInterceptors) UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	ctx = metadata.AppendToOutgoingContext(ctx, modNameMetadataKey, mc.ModName)
	return invoker(ctx, method, req, reply, cc, opts...)
}

// UnaryServerInterceptor checks the incoming RPC metadata for a distributed tracing directive and
// attaches any information to a debug context.
func ModNameUnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	values := meta.Get(modNameMetadataKey)
	if len(values) == 1 {
		ctx = context.WithValue(ctx, modNameKeyID, values[0])
	}

	return handler(ctx, req)
}
