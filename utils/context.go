package utils

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const MetadataKey = "viam-metadata"

func ContextWithMetadata(ctx context.Context) (context.Context, map[string][]string) {
	md := make(map[string][]string)
	ctx = context.WithValue(ctx, MetadataKey, md)
	return ctx, md
}

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
