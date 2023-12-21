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

// GetPosition returns a Pose and a component reference string of the robot's current location according to SLAM.
func (server *serviceServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (
	*pb.GetPositionResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetPosition")
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

// GetPointCloudMap returns the slam service's slam algo's current map state in PCD format as
// a stream of byte chunks.
func (server *serviceServer) GetPointCloudMap(req *pb.GetPointCloudMapRequest,
	stream pb.SLAMService_GetPointCloudMapServer,
) error {
	ctx := context.Background()

	ctx, span := trace.StartSpan(ctx, "slam::server::GetPointCloudMap")
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

// GetInternalState returns the internal state of the slam service's slam algo in a stream of
// byte chunks.
func (server *serviceServer) GetInternalState(req *pb.GetInternalStateRequest,
	stream pb.SLAMService_GetInternalStateServer,
) error {
	ctx := context.Background()
	ctx, span := trace.StartSpan(ctx, "slam::server::GetInternalState")
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

// GetLatestMapInfo returns the timestamp of when the map was last updated.
func (server *serviceServer) GetLatestMapInfo(ctx context.Context, req *pb.GetLatestMapInfoRequest) (
	*pb.GetLatestMapInfoResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetLatestMapInfo")
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

// GetProperties returns the mapping mode and of the slam process and whether it is being done locally
// or in the cloud.
func (server *serviceServer) GetProperties(ctx context.Context, req *pb.GetPropertiesRequest) (
	*pb.GetPropertiesResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetProperties")
	defer span.End()

	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	prop, err := svc.Properties(ctx)
	if err != nil {
		return nil, err
	}

	mappingMode := pb.MappingMode_MAPPING_MODE_UNSPECIFIED
	switch prop.MappingMode {
	case MappingModeNewMap:
		mappingMode = pb.MappingMode_MAPPING_MODE_CREATE_NEW_MAP
	case MappingModeLocalizationOnly:
		mappingMode = pb.MappingMode_MAPPING_MODE_LOCALIZE_ONLY
	case MappingModeUpdateExistingMap:
		mappingMode = pb.MappingMode_MAPPING_MODE_UPDATE_EXISTING_MAP
	}

	return &pb.GetPropertiesResponse{
		CloudSlam:   prop.CloudSlam,
		MappingMode: mappingMode,
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
