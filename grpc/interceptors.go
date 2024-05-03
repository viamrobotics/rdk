package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
)

// defaultMethodTimeout is the default context timeout for all inbound gRPC
// methods used when no deadline is set on the context.
var defaultMethodTimeout = 10 * time.Minute

// EnsureTimeoutUnaryInterceptor sets a default timeout on the context if one is
// not already set. To be called as the first unary server interceptor.
func EnsureTimeoutUnaryInterceptor(ctx context.Context, req interface{},
	info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (interface{}, error) {
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultMethodTimeout)
		defer cancel()
	}

	return handler(ctx, req)
}
