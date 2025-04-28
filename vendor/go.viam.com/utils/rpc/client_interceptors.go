package rpc

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryClientTracingInterceptor adds the current Span's metadata to the context.
func UnaryClientTracingInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		ctx = contextWithSpanMetadata(ctx)
		err := invoker(ctx, method, req, reply, cc, opts...)
		return err
	}
}

// StreamClientTracingInterceptor adds the current Span's metadata to the context.
func StreamClientTracingInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn,
		method string, streamer grpc.Streamer, opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		ctx = contextWithSpanMetadata(ctx)
		stream, err := streamer(ctx, desc, cc, method, opts...)
		return stream, err
	}
}

func contextWithSpanMetadata(ctx context.Context) context.Context {
	span := trace.FromContext(ctx)
	if span == nil {
		return ctx
	}

	ctx = metadata.AppendToOutgoingContext(
		ctx,
		"trace-id", span.SpanContext().TraceID.String(),
		"span-id", span.SpanContext().SpanID.String(),
		"trace-options", fmt.Sprint(span.SpanContext().TraceOptions),
	)
	return ctx
}

// UnaryClientInvalidAuthInterceptor clears the access token stored on creds in
// the event of an "Unauthenticated" or "PermissionDenied" gRPC error to force
// re-auth.
func UnaryClientInvalidAuthInterceptor(creds *perRPCJWTCredentials) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
	) error {
		err := invoker(ctx, method, req, reply, cc, opts...)
		if c := status.Code(err); c == codes.Unauthenticated || c == codes.PermissionDenied &&
			creds != nil {
			creds.accessToken = ""
		}
		return err
	}
}
