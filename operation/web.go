package operation

import (
	"context"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const opidMetadataKey = "opid"

// UnaryClientInterceptor adds the operation id from the current context (if any) to the
// outgoing unary RPC metadata.
func UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *googlegrpc.ClientConn,
	invoker googlegrpc.UnaryInvoker,
	opts ...googlegrpc.CallOption,
) error {
	if op := Get(ctx); op != nil {
		// TODO: error if stringify fails?
		ctx = metadata.AppendToOutgoingContext(ctx, opidMetadataKey, op.ID.String())
	}
	return invoker(ctx, method, req, reply, cc, opts...)
	// TODO: should we something if the server provides a new opid?
}

// StreamClientInterceptor adds the operation id from the current context (if any) to the
// outgoing streaming RPC metadata.
func StreamClientInterceptor(
	ctx context.Context,
	desc *googlegrpc.StreamDesc,
	cc *googlegrpc.ClientConn,
	method string,
	streamer googlegrpc.Streamer,
	opts ...googlegrpc.CallOption,
) (googlegrpc.ClientStream, error) {
	if op := Get(ctx); op != nil {
		// TODO: error if stringify fails?
		ctx = metadata.AppendToOutgoingContext(ctx, opidMetadataKey, op.ID.String())
	}
	return streamer(ctx, desc, cc, method, opts...)
	// TODO: should we something if the server provides a new opid?
}

func (m *Manager) UnaryServerInterceptor(
	ctx context.Context,
	req interface{},
	info *googlegrpc.UnaryServerInfo,
	handler googlegrpc.UnaryHandler,
) (interface{}, error) {
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, errors.New("failed to pull metadata from context")
	}
	opid, err := getOrCreateFromMetadata(meta)
	if err != nil {
		return nil, err
	}
	ctx, done := m.createWithID(ctx, opid, info.FullMethod, nil)
	defer done()
	meta.Set(opidMetadataKey, opid.String())
	googlegrpc.SendHeader(ctx, meta)
	return handler(ctx, req)
}

func (m *Manager) StreamServerInterceptor(
	srv interface{},
	ss googlegrpc.ServerStream,
	info *googlegrpc.StreamServerInfo,
	handler googlegrpc.StreamHandler,
) error {
	// check metadata for opids
	ctx := ss.Context()
	meta, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return errors.New("failed to pull metadata from context")
	}
	opid, err := getOrCreateFromMetadata(meta)
	if err != nil {
		return err
	}
	ctx, done := m.createWithID(ctx, opid, info.FullMethod, nil)
	defer done()
	meta.Set(opidMetadataKey, opid.String())
	ss.SendHeader(meta)

	return handler(srv, &ssStreamContextWrapper{ss, ctx})
}

// getOrCreateFromMetadata returns an operation id from metadata, or generates a random
// UUID if the metadata does not contain any.
func getOrCreateFromMetadata(meta metadata.MD) (uuid.UUID, error) {
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
	googlegrpc.ServerStream
	ctx context.Context
}

func (w ssStreamContextWrapper) Context() context.Context {
	return w.ctx
}
