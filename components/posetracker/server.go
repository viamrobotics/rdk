package posetracker

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/posetracker/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

type subtypeServer struct {
	pb.UnimplementedPoseTrackerServiceServer
	coll resource.SubtypeCollection[PoseTracker]
}

// NewServer constructs a pose tracker gRPC service server.
func NewServer(coll resource.SubtypeCollection[PoseTracker]) pb.PoseTrackerServiceServer {
	return &subtypeServer{coll: coll}
}

func (server *subtypeServer) GetPoses(
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
func (server *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	poseTracker, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, poseTracker, req)
}
