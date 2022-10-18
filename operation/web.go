package operation

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const opidMetadataKey = "opid"

// UnaryClientInterceptor adds the operation id from the current context (if any) to the
// outgoing unary RPC metadata.
func UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	if op := Get(ctx); op != nil && op.ID.String() != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, opidMetadataKey, op.ID.String())
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// StreamClientInterceptor adds the operation id from the current context (if any) to the
// outgoing streaming RPC metadata.
func StreamClientInterceptor(
	ctx context.Context,
	desc *grpc.StreamDesc,
	cc *grpc.ClientConn,
	method string,
	streamer grpc.Streamer,
	opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	if op := Get(ctx); op != nil && op.ID.String() != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, opidMetadataKey, op.ID.String())
	}
	return streamer(ctx, desc, cc, method, opts...)
}

// UnaryServerInterceptor creates a new operation in the current context before passing
// it to the unary response handler. If the incoming RPC metadata contains an operation
// ID, the new operation will have the same ID.
func (m *Manager) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("failed to pull metadata from context")
	}
	opid, err := GetOrCreateFromMetadata(meta)
	if err != nil {
		return nil, err
	}
	ctx, done := m.createWithID(ctx, opid, info.FullMethod, nil)
	defer done()

	return handler(ctx, req)
}

// StreamServerInterceptor creates a new operation in the current context before passing
// it to the stream response handler. If the incoming RPC metadata contains an operation
// ID, the new operation will have the same ID.
func (m *Manager) StreamServerInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	ctx := ss.Context()
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("failed to pull metadata from context")
	}
	opid, err := GetOrCreateFromMetadata(meta)
	if err != nil {
		return err
	}
	ctx, done := m.createWithID(ctx, opid, info.FullMethod, nil)
	defer done()

	return handler(srv, &ssStreamContextWrapper{ss, ctx})
}

// GetOrCreateFromMetadata returns an operation id from metadata, or generates a random
// UUID if the metadata does not contain any.
func GetOrCreateFromMetadata(meta metadata.MD) (uuid.UUID, error) {
	values := meta.Get(opidMetadataKey)
	switch len(values) {
	case 0:
		return uuid.New(), nil
	case 1:
		opid, err := uuid.Parse(values[0])
		if err != nil {
			return uuid.UUID{}, err
		}
		return opid, nil
	default:
		return uuid.UUID{}, errors.New("found more than one operation id in metadata")
	}
}

type ssStreamContextWrapper struct {
	grpc.ServerStream
	ctx context.Context
}

func (w ssStreamContextWrapper) Context() context.Context {
	return w.ctx
}
