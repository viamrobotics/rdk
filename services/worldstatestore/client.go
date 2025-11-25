package worldstatestore

import (
	"context"
	"errors"
	"io"

	commonPB "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.WorldStateStoreServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewWorldStateStoreServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

// ListUUIDs lists all UUIDs in the world state store.
func (c *client) ListUUIDs(ctx context.Context, extra map[string]interface{}) ([][]byte, error) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::client::ListUUIDs")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	req := &pb.ListUUIDsRequest{Name: c.name, Extra: ext}
	resp, err := c.client.ListUUIDs(ctx, req)
	if err != nil {
		return nil, err
	}
	uuids := resp.GetUuids()
	if uuids == nil {
		return nil, ErrNilResponse
	}

	return uuids, nil
}

// GetTransform gets the transform for a given UUID.
func (c *client) GetTransform(ctx context.Context, uuid []byte, extra map[string]interface{}) (*commonPB.Transform, error) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::client::GetTransform")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	req := &pb.GetTransformRequest{Name: c.name, Uuid: uuid, Extra: ext}
	resp, err := c.client.GetTransform(ctx, req)
	if err != nil {
		return nil, err
	}
	obj := resp.GetTransform()
	if obj == nil {
		return nil, ErrNilResponse
	}

	return obj, nil
}

// StreamTransformChanges streams transform changes.
func (c *client) StreamTransformChanges(ctx context.Context, extra map[string]interface{}) (*TransformChangeStream, error) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::client::StreamTransformChanges")
	defer span.End()

	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	req := &pb.StreamTransformChangesRequest{Name: c.name, Extra: ext}
	stream, err := c.client.StreamTransformChanges(ctx, req)
	if err != nil {
		return nil, err
	}
	// Check the initial response immediately to catch early errors.
	_, err = stream.Recv()
	if err != nil {
		return nil, err
	}

	iter := &TransformChangeStream{
		next: func() (TransformChange, error) {
			resp, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return TransformChange{}, io.EOF
				}
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					return TransformChange{}, ctx.Err()
				}
				return TransformChange{}, err
			}
			change := TransformChange{
				ChangeType: resp.ChangeType,
				Transform:  resp.Transform,
			}
			if resp.UpdatedFields != nil {
				change.UpdatedFields = resp.UpdatedFields.Paths
			}
			return change, nil
		},
	}

	return iter, nil
}

// DoCommand handles arbitrary commands.
func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
