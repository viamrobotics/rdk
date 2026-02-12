package worldstatestore

import (
	"context"
	"errors"
	"io"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/utils/trace"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedWorldStateStoreServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a the world state store gRPC service server.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// ListUUIDs returns a list of world state uuids.
func (server *serviceServer) ListUUIDs(ctx context.Context, req *pb.ListUUIDsRequest) (
	*pb.ListUUIDsResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::server::ListUUIDs")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	uuids, err := svc.ListUUIDs(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if uuids == nil {
		return nil, ErrNilResponse
	}

	return &pb.ListUUIDsResponse{Uuids: uuids}, nil
}

// GetTransform returns a world state object by uuid.
func (server *serviceServer) GetTransform(ctx context.Context, req *pb.GetTransformRequest) (
	*pb.GetTransformResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::server::GetTransform")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	obj, err := svc.GetTransform(ctx, req.Uuid, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return &pb.GetTransformResponse{}, nil
	}

	return &pb.GetTransformResponse{Transform: obj}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "worldstatestore::server::DoCommand")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}

// StreamTransformChanges streams changes to world state transforms to the client.
func (server *serviceServer) StreamTransformChanges(
	req *pb.StreamTransformChangesRequest,
	stream pb.WorldStateStoreService_StreamTransformChangesServer,
) error {
	ctx, span := trace.StartSpan(stream.Context(), "worldstatestore::server::StreamTransformChanges")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	changesStream, err := svc.StreamTransformChanges(ctx, req.Extra.AsMap())
	if err != nil {
		return err
	}

	// Send an empty response first so the client doesn't block while checking for errors.
	err = stream.Send(&pb.StreamTransformChangesResponse{})
	if err != nil {
		return err
	}

	for {
		change, err := changesStream.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		// Convert the internal TransformChange to protobuf response
		resp := &pb.StreamTransformChangesResponse{
			ChangeType: change.ChangeType,
			Transform:  change.Transform,
		}

		// Convert UpdatedFields to FieldMask if present
		if len(change.UpdatedFields) > 0 {
			fieldMask := &fieldmaskpb.FieldMask{
				Paths: change.UpdatedFields,
			}
			resp.UpdatedFields = fieldMask
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}
