// Package slam implements simultaneous localization and mapping
// This is an Experimental package
package slam

import (
	"context"
	"io"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/slam/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the SLAMService from the slam proto.
type subtypeServer struct {
	pb.UnimplementedSLAMServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a the slam gRPC service server.
func NewServer(s subtype.Service) pb.SLAMServiceServer {
	return &subtypeServer{subtypeSvc: s}
}

func (server *subtypeServer) service(serviceName string) (Service, error) {
	resource := server.subtypeSvc.Resource(serviceName)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(serviceName))
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

// GetPosition returns a Pose and a component reference string of the robot's current location according to SLAM.
func (server *subtypeServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (
	*pb.GetPositionResponse, error,
) {
	ctx, span := trace.StartSpan(ctx, "slam::server::GetPosition")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}

	p, componentReference, err := svc.GetPosition(ctx, req.Name)
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
func (server *subtypeServer) GetPointCloudMap(req *pb.GetPointCloudMapRequest,
	stream pb.SLAMService_GetPointCloudMapServer,
) error {
	ctx := context.Background()

	ctx, span := trace.StartSpan(ctx, "slam::server::GetPointCloudMap")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return err
	}

	f, err := svc.GetPointCloudMap(ctx, req.Name)
	if err != nil {
		return errors.Wrap(err, "getting callback function from GetPointCloudMap encountered an issue")
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
func (server *subtypeServer) GetInternalState(req *pb.GetInternalStateRequest,
	stream pb.SLAMService_GetInternalStateServer,
) error {
	ctx := context.Background()
	ctx, span := trace.StartSpan(ctx, "slam::server::GetInternalState")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return err
	}

	f, err := svc.GetInternalState(ctx, req.Name)
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

// DoCommand receives arbitrary commands.
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ctx, span := trace.StartSpan(ctx, "slam::server::DoCommand")
	defer span.End()

	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
