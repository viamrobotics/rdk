package posetracker

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/posetracker/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedPoseTrackerServiceServer
	coll resource.APIResourceGetter[PoseTracker]
}

// NewRPCServiceServer constructs a pose tracker gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[PoseTracker], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) GetPoses(
	ctx context.Context,
	req *pb.GetPosesRequest,
) (*pb.GetPosesResponse, error) {
	poseTracker, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	framedPoses, err := poseTracker.Poses(ctx, req.GetBodyNames(), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	poseInFrameProtoStructs := map[string]*commonpb.PoseInFrame{}
	for key, framedPose := range framedPoses {
		framedPoseProto := referenceframe.PoseInFrameToProtobuf(framedPose)
		poseInFrameProtoStructs[key] = framedPoseProto
	}
	return &pb.GetPosesResponse{
		BodyPoses: poseInFrameProtoStructs,
	}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	poseTracker, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, poseTracker, req)
}
