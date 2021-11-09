package config

import (
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"
)

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// Name is the robot part name, FrameConfig gives the general structure of the frame system,
// and ModelFrameConfig is an optional JSON btye stream that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	Name             string
	FrameConfig      *Frame
	ModelFrameConfig []byte
}

// ToProtobuf turns all the interfaces into serializable types
func (part *FrameSystemPart) ToProtobuf() *pb.FrameSystemConfig {
	if part.FrameConfig == nil {
		return nil
	}
	pose := part.FrameConfig.Pose()
	frameConfig := &pb.FrameConfig{Parent: part.FrameConfig.Parent, Pose: spatialmath.PoseToProtobuf(pose)}
	return &pb.FrameSystemConfig{
		Name:        part.Name,
		FrameConfig: frameConfig,
		ModelJson:   part.ModelFrameConfig,
	}
}

// ProtobufToFrameSystemPart takes a protobuf object and transforms it into a FrameSystemPart
func ProtobufToFrameSystemPart(fsc *pb.FrameSystemConfig) *FrameSystemPart {
	if fsc == nil {
		return nil
	}
	pose := spatialmath.NewPoseFromProtobuf(fsc.FrameConfig.Pose)
	point := pose.Point()
	translation := Translation{X: point.X, Y: point.Y, Z: point.Z}
	frameConfig := &Frame{
		Parent:      fsc.FrameConfig.Parent,
		Translation: translation,
		Orientation: pose.Orientation(),
	}
	return &FrameSystemPart{
		Name:             fsc.Name,
		FrameConfig:      frameConfig,
		ModelFrameConfig: fsc.ModelJson,
	}
}
