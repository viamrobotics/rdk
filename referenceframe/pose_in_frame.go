package referenceframe

import (
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/spatialmath"
)

// PoseInFrame is a data structure that packages a pose with the name of the
// frame in which it was observed.
type PoseInFrame struct {
	frame string
	pose  spatialmath.Pose
}

// FrameName returns the name of the frame in which the pose was observed.
func (pF *PoseInFrame) FrameName() string {
	return pF.frame
}

// Pose returns the pose that was observed.
func (pF *PoseInFrame) Pose() spatialmath.Pose {
	return pF.pose
}

// NewPoseInFrame generates a new PoseInFrame.
func NewPoseInFrame(frame string, pose *spatialmath.Pose) *PoseInFrame {
	return &PoseInFrame{
		frame: frame,
		pose:  *pose,
	}
}

// PoseInFrameToProtobuf converts a PoseInFrame struct to a
// PoseInFrame message as specified in common.proto.
func PoseInFrameToProtobuf(framedPose *PoseInFrame) *commonpb.PoseInFrame {
	poseProto := spatialmath.PoseToProtobuf(framedPose.pose)
	return &commonpb.PoseInFrame{
		Frame: framedPose.frame,
		Pose:  poseProto,
	}
}

// ProtobufToPoseInFrame converts a PoseInFrame message as specified in
// common.proto to a PoseInFrame struct.
func ProtobufToPoseInFrame(proto *commonpb.PoseInFrame) *PoseInFrame {
	result := &PoseInFrame{}
	result.pose = spatialmath.NewPoseFromProtobuf(proto.GetPose())
	result.frame = proto.GetFrame()
	return result
}
