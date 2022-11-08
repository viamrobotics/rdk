package posetracker

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/posetracker/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

type subtypeServer struct {
	pb.UnimplementedPoseTrackerServiceServer
	service subtype.Service
}

// NewServer constructs a pose tracker gRPC service server.
func NewServer(service subtype.Service) pb.PoseTrackerServiceServer {
	return &subtypeServer{service: service}
}

// NewResourceIsNotPoseTracker returns an error for when a resource
// does not properly implement the PoseTracker interface.
func NewResourceIsNotPoseTracker(resourceName string) error {
	return errors.Errorf("resource with name (%s) is not a pose tracker", resourceName)
}

// getPoseTracker returns the specified pose tracker (or nil).
func (server *subtypeServer) getPoseTracker(name string) (PoseTracker, error) {
	resource := server.service.Resource(name)
	if resource == nil {
		return nil, utils.NewResourceNotFoundError(Named(name))
	}
	poseTracker, ok := resource.(PoseTracker)
	if !ok {
		return nil, NewResourceIsNotPoseTracker(name)
	}
	return poseTracker, nil
}

func (server *subtypeServer) GetPoses(
	ctx context.Context,
	req *pb.GetPosesRequest,
) (*pb.GetPosesResponse, error) {
	poseTracker, err := server.getPoseTracker(req.GetName())
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
