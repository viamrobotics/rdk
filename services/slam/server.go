package slam

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// serviceServer implements the SLAMService from the slam proto.
type serviceServer struct {
	pb.UnimplementedSLAMServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a the slam gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

// Position returns a Pose and a component reference string of the robot's current location according to SLAM.
func (server *serviceServer) Position(ctx context.Context, req *pb.GetPositionRequest) (
	*pb.GetPositionResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::Position")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, componentReference, err := svc.Position(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GetPositionResponse{
		Pose:               spatialmath.PoseToProtobuf(p),
		ComponentReference: componentReference,
	}, nil
}

// PointCloudMap returns the slam service's slam algo's current map state in PCD format as
// a stream of byte chunks.
func (server *serviceServer) PointCloudMap(req *pb.GetPointCloudMapRequest,
	stream pb.SLAMService_GetPointCloudMapServer,
) error {
	ctx := context.Background()

	ctx, span := trace.StartSpan(ctx, "slam::server::PointCloudMap")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	f, err := svc.PointCloudMap(ctx)
	if err != nil {
		return errors.Wrap(err, "getting callback function from PointCloudMap encountered an issue")
	}

	// In the future, channel buffer could be used here to optimize for latency
	for {
		rawChunk, err := f()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "getting data from callback function encountered an issue")
		}

		chunk := &pb.GetPointCloudMapResponse{PointCloudPcdChunk: rawChunk}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
}

// InternalState returns the internal state of the slam service's slam algo in a stream of
// byte chunks.
func (server *serviceServer) InternalState(req *pb.GetInternalStateRequest,
	stream pb.SLAMService_GetInternalStateServer,
) error {
	ctx := context.Background()
	ctx, span := trace.StartSpan(ctx, "slam::server::InternalState")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	f, err := svc.InternalState(ctx)
	if err != nil {
		return err
	}

	// In the future, channel buffer could be used here to optimize for latency
	for {
		rawChunk, err := f()

		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "getting data from callback function encountered an issue")
		}

		chunk := &pb.GetInternalStateResponse{InternalStateChunk: rawChunk}
		if err := stream.Send(chunk); err != nil {
			return err
		}
	}
}

// LatestMapInfo returns the timestamp of when the map was last updated.
func (server *serviceServer) LatestMapInfo(ctx context.Context, req *pb.GetLatestMapInfoRequest) (
	*pb.GetLatestMapInfoResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::LatestMapInfo")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	mapTimestamp, err := svc.LatestMapInfo(ctx)
	if err != nil {
		return nil, err
	}
	protoTimestamp := timestamppb.New(mapTimestamp)

	return &pb.GetLatestMapInfoResponse{
		LastMapUpdate: protoTimestamp,
	}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "slam::server::DoCommand")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
