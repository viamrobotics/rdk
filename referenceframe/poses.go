package referenceframe

import (
	"github.com/pkg/errors"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/rdk/spatialmath"
)

// FrameSystemPoses is an alias for a mapping of frame names to PoseInFrame.
type FrameSystemPoses map[string]*PoseInFrame

// FrameSystemPosesToProto converts a FrameSystemPoses to its representation in protobuf.
func FrameSystemPosesToProto(ps FrameSystemPoses) *pb.PlanStep {
	step := make(map[string]*pb.ComponentState)
	for name, pose := range ps {
		pbPose := spatialmath.PoseToProtobuf(pose.Pose())
		step[name] = &pb.ComponentState{Pose: pbPose}
	}
	return &pb.PlanStep{Step: step}
}

// FrameSystemPosesFromProto converts a *pb.PlanStep to a PlanStep.
func FrameSystemPosesFromProto(ps *pb.PlanStep) (FrameSystemPoses, error) {
	if ps == nil {
		return FrameSystemPoses{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(FrameSystemPoses, len(ps.Step))
	for k, v := range ps.Step {
		step[k] = NewPoseInFrame(World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
}
