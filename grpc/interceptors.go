package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
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
