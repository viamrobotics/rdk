// Package motion contains a gRPC based motion service server
package motion

import (
	"context"

	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

// subtypeServer implements the MotionService from motion.proto.
type subtypeServer struct {
	pb.UnimplementedMotionServiceServer
	subtypeSvc subtype.Service
}

// NewServer constructs a motion gRPC service server.
func NewServer(s subtype.Service) pb.MotionServiceServer {
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

func (server *subtypeServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	worldState, err := referenceframe.WorldStateFromProtobuf(req.GetWorldState())
	if err != nil {
		return nil, err
	}
	success, err := svc.Move(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		referenceframe.ProtobufToPoseInFrame(req.GetDestination()),
		worldState,
		req.Extra.AsMap(),
	)
	return &pb.MoveResponse{Success: success}, err
}

func (server *subtypeServer) MoveSingleComponent(
	ctx context.Context,
	req *pb.MoveSingleComponentRequest,
) (*pb.MoveSingleComponentResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	worldState, err := referenceframe.WorldStateFromProtobuf(req.GetWorldState())
	if err != nil {
		return nil, err
	}
	success, err := svc.MoveSingleComponent(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		referenceframe.ProtobufToPoseInFrame(req.GetDestination()),
		worldState,
		req.Extra.AsMap(),
	)
	return &pb.MoveSingleComponentResponse{Success: success}, err
}

func (server *subtypeServer) GetPose(ctx context.Context, req *pb.GetPoseRequest) (*pb.GetPoseResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	if req.ComponentName == nil {
		return nil, errors.New("must provide component name")
	}
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	pose, err := svc.GetPose(ctx, protoutils.ResourceNameFromProto(req.ComponentName), req.DestinationFrame, transforms, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(pose)}, nil
}
