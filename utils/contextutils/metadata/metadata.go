// Package metadata implements Client-to-Server arbitrary metadata passing via Context.
package metadata

import (
	"context"
	"iter"
	"maps"
	"strings"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"google.golang.org/grpc"
	grpcmetadata "google.golang.org/grpc/metadata"
)

type arbitraryMetadataContextKey struct{}

// arbitraryMetadataKeyPrefix helps differentiate between metadata keys provided by the user and those for internal use.
// It is automatically prefixed to all arbitrary keys before serialization and removed after deserialization to avoid conflicting
// with internal metadata.
const arbitraryMetadataKeyPrefix = "viam-metadata-"

// ViamMD is a mapping from metadata keys to values.
type ViamMD map[string]string

// Set returns a new derived context containing the key-value pairs as metadata.
func Set(ctx context.Context, kv ...string) context.Context {
	if len(kv) == 0 {
		return ctx
	}

	var md ViamMD
	var ok bool
	if md, ok = FromContext(ctx); !ok {
		md = make(ViamMD)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		md[kv[i]] = kv[i+1]
	}
	return context.WithValue(ctx, arbitraryMetadataContextKey{}, md)
}

// Get returns the metadata value associated with the provided key, if found.
func Get(ctx context.Context, key string) (string, bool) {
	if md, ok := ctx.Value(arbitraryMetadataContextKey{}).(ViamMD); ok {
		if v, ok := md[key]; ok {
			return v, true
		}
	}
	return "", false
}

// Delete returns a new derived context without the metadata associated with the provided keys.
func Delete(ctx context.Context, keys ...string) context.Context {
	if len(keys) == 0 {
		return ctx
	}

	var md ViamMD
	var ok bool
	if md, ok = FromContext(ctx); ok {
		for _, key := range keys {
			delete(md, key)
		}
		return context.WithValue(ctx, arbitraryMetadataContextKey{}, md)
	}
	return ctx
}

// FromContext returns a clone of the metadata map in the context.
func FromContext(ctx context.Context) (map[string]string, bool) {
	if md, ok := ctx.Value(arbitraryMetadataContextKey{}).(ViamMD); ok {
		return maps.Clone(md), true
	}
	return nil, false
}

// All returns an iterator of the metadata keys and values.
func All(ctx context.Context) iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		if md, ok := ctx.Value(arbitraryMetadataContextKey{}).(ViamMD); ok {
			for k, v := range md {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// ViamClientToServerMetadataUnaryClientInterceptor converts the metadata map in the context to our wire format and adds it with
// metadata.AppendToOutgoingContext.
func ViamClientToServerMetadataUnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if md, ok := ctx.Value(arbitraryMetadataContextKey{}).(ViamMD); ok {
		kvPairs := make([]string, 0, len(md))
		for k, v := range md {
			kvPairs = append(kvPairs, arbitraryMetadataKeyPrefix+k, v)
		}
		if len(kvPairs) > 0 {
			ctx = grpcmetadata.AppendToOutgoingContext(ctx, kvPairs...)
		}
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// ViamClientToServerMetadataStreamClientInterceptor converts the metadata map in the context to our wire format and adds it with
// // metadata.AppendToOutgoingContext.
func ViamClientToServerMetadataStreamClientInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if md, ok := ctx.Value(arbitraryMetadataContextKey{}).(ViamMD); ok {
		kvPairs := make([]string, 0, len(md))
		for k, v := range md {
			kvPairs = append(kvPairs, arbitraryMetadataKeyPrefix+k, v)
		}
		if len(kvPairs) > 0 {
			ctx = grpcmetadata.AppendToOutgoingContext(ctx, kvPairs...)
		}
	}
	return streamer(ctx, desc, cc, method, opts...)
}

// ViamClientToServerMetadataUnaryServerInterceptor retrieves the metadata map using metadata.FromIncomingContext and converts from
// it from our wire format the local format.
func ViamClientToServerMetadataUnaryServerInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	if md, ok := grpcmetadata.FromIncomingContext(ctx); ok {
		var kvPairs []string
		for prefixedKey, vs := range md {
			if strings.HasPrefix(prefixedKey, arbitraryMetadataKeyPrefix) {
				// grpc metadata is map[string][]string, but ours is map[string]string.
				// in the unlikely case that vs has more than 1 element, Set will take the last.
				for _, v := range vs {
					kvPairs = append(kvPairs, strings.TrimPrefix(prefixedKey, arbitraryMetadataKeyPrefix), v)
				}
			}
		}
		if len(kvPairs) > 0 {
			ctx = Set(ctx, kvPairs...)
		}
	}
	return handler(ctx, req)
}

// ViamClientToServerMetadataStreamServerInterceptor retrieves the metadata map using metadata.FromIncomingContext and converts from
// it from our wire format the local format.
func ViamClientToServerMetadataStreamServerInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx := ss.Context()
	if md, ok := grpcmetadata.FromIncomingContext(ctx); ok {
		var kvPairs []string
		for prefixedKey, vs := range md {
			if strings.HasPrefix(prefixedKey, arbitraryMetadataKeyPrefix) {
				// grpc metadata is map[string][]string, but ours is map[string]string. if vs has more than 1 element, Set will take the last.
				for _, v := range vs {
					kvPairs = append(kvPairs, strings.TrimPrefix(prefixedKey, arbitraryMetadataKeyPrefix), v)
				}
			}
		}
		if len(kvPairs) > 0 {
			ctx = Set(ctx, kvPairs...)
			wrapped := &grpc_middleware.WrappedServerStream{ServerStream: ss, WrappedContext: ctx}
			return handler(srv, wrapped)
		}
	}
	return handler(srv, ss)
}
