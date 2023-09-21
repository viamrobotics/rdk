// Package contextutils provides utilities for dealing with contexts such as adding and
// retrieving metadata to/from a context, and handling context timeouts.
package contextutils

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type contextKey string

const (
	// MetadataContextKey is the key used to access metadata from a context with metadata.
	MetadataContextKey = contextKey("viam-metadata")

	// TimeRequestedMetadataKey is optional metadata in the gRPC response header that correlates
	// to the time right before the point cloud was captured.
	TimeRequestedMetadataKey = "viam-time-requested"

	// TimeReceivedMetadataKey is optional metadata in the gRPC response header that correlates
	// to the time right after the point cloud was captured.
	TimeReceivedMetadataKey = "viam-time-received"
)

// ContextWithMetadata attaches a metadata map to the context.
func ContextWithMetadata(ctx context.Context) (context.Context, map[string][]string) {
	// If the context already has metadata, return that and leave the context untouched.
	existingMD := ctx.Value(MetadataContextKey)
	if mdMap, ok := existingMD.(map[string][]string); ok {
		return ctx, mdMap
	}

	// Otherwise, add a metadata map to the context.
	md := make(map[string][]string)
	ctx = context.WithValue(ctx, MetadataContextKey, md)
	return ctx, md
}

// ContextWithMetadataUnaryClientInterceptor attempts to read metadata from the gRPC header and
// injects the metadata into the context if the caller has passed in a context with metadata.
func ContextWithMetadataUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var header metadata.MD
	opts = append(opts, grpc.Header(&header))
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		return err
	}

	md := ctx.Value(MetadataContextKey)
	if mdMap, ok := md.(map[string][]string); ok {
		for key, value := range header {
			mdMap[key] = value
		}
	}

	return nil
}

// ContextWithTimeoutIfNoDeadline returns a child timeout context derived from `ctx`
// only if `ctx` doesn't already have a deadline set. Returns `ctx` if `ctx` already has a deadline
func ContextWithTimeoutIfNoDeadline(ctx context.Context, timeout time.Duration) context.Context {
	if _, ok := ctx.Deadline(); !ok {
		newTimeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		ctx = newTimeoutCtx
		defer cancel()
	}
	return ctx
}
