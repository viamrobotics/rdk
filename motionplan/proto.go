package motionplan

import (
	"errors"

	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// FrameSystemPosesToProto converts a referenceframe.FrameSystemPoses to its representation in protobuf.
func FrameSystemPosesToProto(ps referenceframe.FrameSystemPoses) *pb.PlanStep {
	step := make(map[string]*pb.ComponentState)
	for name, pose := range ps {
		pbPose := spatialmath.PoseToProtobuf(pose.Pose())
		step[name] = &pb.ComponentState{Pose: pbPose}
	}
	return &pb.PlanStep{Step: step}
}

// FrameSystemPosesFromProto converts a *pb.PlanStep to a PlanStep.
func FrameSystemPosesFromProto(ps *pb.PlanStep) (referenceframe.FrameSystemPoses, error) {
	if ps == nil {
		return referenceframe.FrameSystemPoses{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(referenceframe.FrameSystemPoses, len(ps.Step))
	for k, v := range ps.Step {
		step[k] = referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromProtobuf(v.Pose))
	}
	return step, nil
}
