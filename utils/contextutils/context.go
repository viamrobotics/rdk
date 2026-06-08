// Package contextutils provides utilities for dealing with contexts such as adding and
// retrieving metadata to/from a context, and handling context timeouts.
package contextutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

	// arbitraryMetadataKey records the metadata keys provided by the user (as opposed to those for internal use)
	// It is automatically prefixed to all arbitrary keys behind the scenes to avoid conflicts with internal metadata.
	arbitraryMetadataKey = string(MetadataContextKey)

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

// Deprecated: ContextWithMetadata attaches a metadata map to the context.
// Instead, to read arbitrary viam-metadata from server, use
//
//	md := make(contextutils.MD)
//	ctx = context.WithValue(ctx, MetadataContextKey, md)
//	resp, err := someRPCCall(ctx, ...)
//	for k, v := range md {...}
//
//nolint:revive // ignore exported comment check
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

// Deprecated: ContextWithMetadataUnaryClientInterceptorDeprecated attempts to read metadata from the gRPC header and
// injects the metadata into the context if the caller has passed in a context with metadata.
// It is only to be used with the also deprecated ContextWithMetadata. It does not work across modules.
// Instead use ContextWithMetadataServerToClientUnaryClientInterceptor.
//
//nolint:revive // ignore exported comment check
func ContextWithMetadataUnaryClientInterceptorDeprecated(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var header grpcmetadata.MD
	if ctx.Value(MetadataContextKey) != nil {
		opts = append(opts, grpc.Header(&header))
	}
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

// ContextWithMetadataServerToClientUnaryClientInterceptor attempts to read metadata from the gRPC header and
// injects the metadata into the context if the caller has passed in a context with metadata.
func ContextWithMetadataServerToClientUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	var header grpcmetadata.MD
	md := ctx.Value(MetadataContextKey)
	if _, ok := md.(ViamMD); ok {
		opts = append(opts, grpc.Header(&header))
	}
	err := invoker(ctx, method, req, reply, cc, opts...)
	if err != nil {
		return err
	}

	if len(header) > 0 {
		if mdMap, ok := md.(ViamMD); ok {
			for _, prefixedKeys := range header.Get(arbitraryMetadataKey) {
				if v := header.Get(prefixedKeys); len(v) > 0 {
					mdMap[strings.TrimPrefix(prefixedKeys, arbitraryMetadataKey+"-")] = v
				}
			}
		}
	}

	return nil
}

// ContextWithMetadataServerToClientUnaryServerInterceptor upgrades the incoming context to a ContextWithMetadata,
// before calling the handler function. After, it sets the header metadata to the metadata map (if any).
func ContextWithMetadataServerToClientUnaryServerInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	md := ViamMD{}
	ctx = context.WithValue(ctx, MetadataContextKey, md)
	resp, err := handler(ctx, req)
	if len(md) > 0 {
		wire := toWireMD(md)
		_ = grpc.SetHeader(ctx, wire) //nolint:errcheck
	}
	return resp, err
}

// toWireMD transforms the incoming ViamMD to a new one where all keys are previxed with arbitraryMetadataKey-
// and adds a new mapping of arbitraryMetadataKey: [prefixedKeys]
func toWireMD(md map[string][]string) grpcmetadata.MD {
	wireMD := grpcmetadata.MD{}
	prefixedKeys := make([]string, 0, len(md))
	for k, v := range md {
		wireKey := arbitraryMetadataKey + "-" + k
		wireMD[wireKey] = v
		prefixedKeys = append(prefixedKeys, wireKey)
	}
	if len(prefixedKeys) > 0 {
		wireMD[arbitraryMetadataKey] = prefixedKeys
	}
	return wireMD
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

	// dedupe key list to prevent duplicative appends with multiple hops
	keys := slices.Clone(md.Get(arbitraryMetadataKey))
	slices.Sort(keys)
	keys = slices.Compact(keys)

	for _, prefixedKeys := range keys {
		ctx = grpcmetadata.AppendToOutgoingContext(ctx, arbitraryMetadataKey, prefixedKeys)
		for _, val := range md.Get(prefixedKeys) {
			ctx = grpcmetadata.AppendToOutgoingContext(ctx, prefixedKeys, val)
		}
	}
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

// appendToOutgoingContext functions like metadata.AppendToOutgoingContext, but also tracks the unique list of arbitrary keys
// under the arbitraryMetadataKey key.
func appendToOutgoingContext(ctx context.Context, kv ...string) context.Context {
	seenKeys := make(map[string]struct{})
	arbitraryKeys := make([]string, 0, len(kv))
	prefixedPairs := make([]string, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		wireKey := arbitraryMetadataKey + "-" + kv[i]
		prefixedPairs[i] = wireKey
		prefixedPairs[i+1] = kv[i+1]
		if _, dup := seenKeys[kv[i]]; dup {
			continue
		}
		seenKeys[kv[i]] = struct{}{}
		arbitraryKeys = append(arbitraryKeys, arbitraryMetadataKey, wireKey)
	}
	ctx = grpcmetadata.AppendToOutgoingContext(ctx, prefixedPairs...)
	return grpcmetadata.AppendToOutgoingContext(ctx, arbitraryKeys...)
}

// AppendMetadata appends Viam arbitrary metadata to the context.
func AppendMetadata(ctx context.Context, kv ...string) context.Context {
	return appendToOutgoingContext(ctx, kv...)
}

// Metadata retrieves the Viam arbitrary metadata from the context.
func Metadata(ctx context.Context) (ViamMD, bool) {
	// RPC
	if md, ok := fromIncomingContext(ctx); ok {
		return md, true
	}
	// local
	if md, ok := fromOutgoingContext(ctx); ok {
		return md, true
	}
	return ViamMD{}, false
}

// fromIncomingContext functions like metadata.FromIncomingContext but strips the prefix added by appendToOutgoingContext.
func fromIncomingContext(ctx context.Context) (ViamMD, bool) {
	incomingMD, ok := grpcmetadata.FromIncomingContext(ctx)
	if !ok {
		return nil, false
	}
	md := ViamMD{}
	for k, v := range incomingMD {
		if strings.HasPrefix(k, arbitraryMetadataKey+"-") {
			md[strings.TrimPrefix(k, arbitraryMetadataKey+"-")] = v
		}
	}
	return md, true
}

// fromOutgoingContext functions like metadata.FromOutgoingContext but strips the prefix added by appendToOutgoingContext.
func fromOutgoingContext(ctx context.Context) (ViamMD, bool) {
	outgoingMD, ok := grpcmetadata.FromOutgoingContext(ctx)
	if !ok {
		return nil, false
	}
	md := ViamMD{}
	for k, v := range outgoingMD {
		if strings.HasPrefix(k, arbitraryMetadataKey+"-") {
			md[strings.TrimPrefix(k, arbitraryMetadataKey+"-")] = v
		}
	}
	return md, true
}

// SetHeader functions like grpc.SetHeader, but also tracks the unique list of arbitrary keys under the arbitraryMetadataKey key
// and prepends keys with the prefix arbitraryMetadataKey- to allow shadowing internal keys.
func SetHeader(ctx context.Context, md ViamMD) error {
	wireMD := grpcmetadata.MD{}
	for k, v := range md {
		wireKey := arbitraryMetadataKey + "-" + k
		wireMD[wireKey] = v
		wireMD.Append(arbitraryMetadataKey, wireKey)
	}
	return grpc.SetHeader(ctx, wireMD)
}

// SendHeader functions like grpc.SendHeader, but also tracks the unique list of arbitrary keys under the arbitraryMetadataKey key
// and prepends keys with the prefix arbitraryMetadataKey- to allow shadowing internal keys.
func SendHeader(ctx context.Context, md grpcmetadata.MD) error {
	wireMD := grpcmetadata.MD{}
	for k, v := range md {
		wireKey := arbitraryMetadataKey + "-" + k
		wireMD[wireKey] = v
		wireMD.Append(arbitraryMetadataKey, wireKey)
	}
	return grpc.SendHeader(ctx, wireMD)
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
