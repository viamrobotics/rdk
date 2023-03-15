// Package motion contains a gRPC based motion service server
package motion

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
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
	var slamServiceName resource.Name
	if protoName := req.GetSlamServiceName(); protoName != nil {
		slamServiceName = protoutils.ResourceNameFromProto(protoName)
	} else {
		slamServiceName = slam.Named("")
	}
	success, err := svc.Move(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		referenceframe.ProtobufToPoseInFrame(req.GetDestination()),
		worldState,
		req.GetConstraints(),
		slamServiceName,
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

// DoCommand receives arbitrary commands.
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.service(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
