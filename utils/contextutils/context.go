// Package contextutils provides utilities for dealing with contexts such as adding and
// retrieving metadata to/from a context, and handling context timeouts.
package contextutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"go.viam.com/rdk/utils"
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

	readTimeout            = 5 * time.Second
	readTimeoutBehindProxy = time.Minute
	initialReadTimeout     = 1 * time.Second
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

// ContextWithTimeoutIfNoDeadline returns a child timeout context derived from `ctx` if a
// deadline does not exist. Returns a cancel context and cancel func from `ctx` if deadline exists.
func ContextWithTimeoutIfNoDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

// GetTimeoutCtx returns a context [and its cancel function] with a timeout value determined by whether we are behind a proxy and whether a
// cached config exists.
func GetTimeoutCtx(ctx context.Context, shouldReadFromCache bool, id string) (context.Context, func()) {
	timeout := readTimeout
	// When environment indicates we are behind a proxy, bump timeout. Network
	// operations tend to take longer when behind a proxy.
	if proxyAddr := os.Getenv(rpc.SocksProxyEnvVar); proxyAddr != "" {
		timeout = readTimeoutBehindProxy
	}

	// use shouldReadFromCache to determine whether this is part of initial read or not, but only shorten timeout
	// if cached config exists
	cachedConfigExists := false
	cloudCacheFilepath := fmt.Sprintf("cached_cloud_config_%s.json", id)
	if _, err := os.Stat(filepath.Join(utils.ViamDotDir, cloudCacheFilepath)); err == nil {
		cachedConfigExists = true
	}
	if shouldReadFromCache && cachedConfigExists {
		timeout = initialReadTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
