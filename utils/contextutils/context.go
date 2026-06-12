// Package contextutils provides utilities for dealing with contexts such as adding and
// retrieving metadata to/from a context, and handling context timeouts.
package contextutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/utils"
)

type contextKey string

const (
	// MetadataContextKey is the key used to access metadata from a context with metadata.
	MetadataContextKey = contextKey("viam-metadata")

	// arbitraryMetadataKeyPrefix helps differentiate between metadata keys provided by the user and those for internal use.
	// It is automatically prefixed to all arbitrary keys behind the scenes to avoid conflicts with internal metadata.
	arbitraryMetadataKeyPrefix = string(MetadataContextKey) + "-"

	// TimeRequestedMetadataKey is optional metadata in the gRPC response header that correlates
	// to the time right before the point cloud was captured.
	TimeRequestedMetadataKey = "viam-time-requested"

	// TimeReceivedMetadataKey is optional metadata in the gRPC response header that correlates
	// to the time right after the point cloud was captured.
	TimeReceivedMetadataKey = "viam-time-received"

	// Timeout values to use when reading a config either from App behind a proxy, or from App with a local (cached) file.
	// The timeout is far shorter when a cached config exists because the machine can always fall back to the cached config.
	readConfigFromCloudBehindProxyTimeout = time.Minute
	readCachedConfigTimeout               = 1 * time.Second
)

// ViamMD is a mapping from metadata keys to values.
type ViamMD map[string][]string

// Pairs is a helper function like metadata.Pairs.
func Pairs(kv ...string) ViamMD {
	return ViamMD(grpcmetadata.Pairs(kv...))
}

// ContextWithMetadata creates a new derived context with a metadata map attached, or if the existing context already contains an MD map,
// returns it along with its attached map.
//
//	Notes:
//	a) do not make concurrent RPC calls using the context without synchronization, or you may get a fatal concurrent map write error.
//	b) this will not work across modules
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

// ContextWithMetadataUnaryClientInterceptor unconditionally adds a header read request to the RPC invoke options
// and, if the caller has passed in a context with an attached metadata map, adds all returned headers into it.
func ContextWithMetadataUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var header grpcmetadata.MD
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

// ContextWithMetadataClientToServerUnaryServerInterceptor retrieves metadata from the incoming context and appends to the outgoing context.
func ContextWithMetadataClientToServerUnaryServerInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	md, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return handler(ctx, req)
	}

	prefixedPairs := make([]string, 0, len(md))
	for prefixedKey, vals := range md {
		if strings.HasPrefix(prefixedKey, arbitraryMetadataKeyPrefix) {
			for _, v := range vals {
				prefixedPairs = append(prefixedPairs, prefixedKey, v)
			}
		}
	}
	ctx = grpcmetadata.AppendToOutgoingContext(ctx, prefixedPairs...)

	return handler(ctx, req)
}

// ContextWithTimeoutIfNoDeadline returns a child timeout context derived from `ctx` if a
// deadline does not exist. Returns a cancel context and cancel func from `ctx` if deadline exists.
func ContextWithTimeoutIfNoDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

// AppendMetadata returns a new context with the Viam arbitrary metadata appended.
func AppendMetadata(ctx context.Context, kv ...string) context.Context {
	prefixedPairs := make([]string, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		prefixedPairs[i] = arbitraryMetadataKeyPrefix + kv[i]
		prefixedPairs[i+1] = kv[i+1]
	}
	return grpcmetadata.AppendToOutgoingContext(ctx, prefixedPairs...)
}

// ReplaceMetadata returns a new context with old Viam arbitrary metadata cleared and new metadata added.
func ReplaceMetadata(ctx context.Context, kv ...string) context.Context {
	prefixedPairs := make([]string, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		prefixedPairs[i] = arbitraryMetadataKeyPrefix + kv[i]
		prefixedPairs[i+1] = kv[i+1]
	}
	return grpcmetadata.NewOutgoingContext(ctx, grpcmetadata.Pairs(prefixedPairs...))
}

// Metadata retrieves the Viam arbitrary metadata from the context.
func Metadata(ctx context.Context) (ViamMD, bool) {
	// ServerInterceptor already forwards MD's from FromIncomingContext to AppendToOutgoingContext, so we can use fromOutgoingContext here.
	if md, ok := fromOutgoingContext(ctx); ok {
		return md, true
	}
	return ViamMD{}, false
}

// fromOutgoingContext functions like metadata.FromOutgoingContext but strips the prefix added by appendToOutgoingContext.
func fromOutgoingContext(ctx context.Context) (ViamMD, bool) {
	outgoingMD, ok := grpcmetadata.FromOutgoingContext(ctx)
	if !ok {
		return nil, false
	}
	md := ViamMD{}
	for k, v := range outgoingMD {
		if strings.HasPrefix(k, arbitraryMetadataKeyPrefix) {
			md[strings.TrimPrefix(k, arbitraryMetadataKeyPrefix)] = v
		}
	}
	return md, true
}

// GetTimeoutCtx returns a context [and its cancel function] with a timeout value determined by whether an environment variable is set,
// we are behind a proxy and whether a cached config exists. The timeout will always use the environment variable if set.
func GetTimeoutCtx(ctx context.Context, shouldReadFromCache bool, id string, logger logging.Logger) (context.Context, func()) {
	timeout, isDefault := utils.GetConfigReadTimeout(logger)
	if !isDefault {
		return context.WithTimeout(ctx, timeout)
	}
	// When environment indicates we are behind a proxy, bump timeout. Network
	// operations tend to take longer when behind a proxy.
	if proxyAddr := os.Getenv(rpc.SocksProxyEnvVar); proxyAddr != "" {
		timeout = readConfigFromCloudBehindProxyTimeout
	}

	// use shouldReadFromCache to determine whether this is part of initial read or not, but only shorten timeout
	// if cached config exists
	cachedConfigExists := false
	cloudCacheFilepath := fmt.Sprintf("cached_cloud_config_%s.json", id)
	if _, err := os.Stat(filepath.Join(utils.ViamDotDir, cloudCacheFilepath)); err == nil {
		cachedConfigExists = true
	}
	if shouldReadFromCache && cachedConfigExists {
		timeout = readCachedConfigTimeout
	}
	return context.WithTimeout(ctx, timeout)
}
