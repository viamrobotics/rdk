package contextutils

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	// MetadataKey is the key used to access metadata from a context with metadata.
	MetadataKey = "viam-metadata"

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
	existingMD := ctx.Value(MetadataKey)
	if mdMap, ok := existingMD.(map[string][]string); ok {
		return ctx, mdMap
	}

	// Otherwise, add a metadata map to the context.
	md := make(map[string][]string)
	ctx = context.WithValue(ctx, MetadataKey, md)
	return ctx, md
}

// ContextWithMetadataUnaryClientInterceptor attempst to read metadata from the gRPC header and
// injects the metadata into the context if the caller has passed in a context with metadata.
func ContextWithMetadataUnaryClientInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var header metadata.MD
	opts = append(opts, grpc.Header(&header))
	invoker(ctx, method, req, reply, cc, opts...)

	md := ctx.Value(MetadataKey)
	if mdMap, ok := md.(map[string][]string); ok {
		for key, value := range header {
			mdMap[key] = value
		}
	}

	return nil
}
