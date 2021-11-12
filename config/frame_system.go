package config

import (
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/spatialmath"
)

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// Name is the robot part name, FrameConfig gives the general structure of the frame system,
// and ModelFrameConfig is an optional ModelJSON that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	Name        string
	FrameConfig *Frame
	ModelFrame  *referenceframe.Model
}

// ToProtobuf turns all the interfaces into serializable types
func (part *FrameSystemPart) ToProtobuf() (*pb.FrameSystemConfig, error) {
	if part.FrameConfig == nil {
		return nil, nil
	}
	pose := part.FrameConfig.Pose()
	frameConfig := &pb.FrameConfig{Parent: part.FrameConfig.Parent, Pose: spatialmath.PoseToProtobuf(pose)}
	var modelJSON []byte
	var err error
	if part.ModelFrame != nil {
		modelJSON, err = part.ModelFrame.MarshalJSON()
		if err != nil {
			return nil, err
		}
	}
	return &pb.FrameSystemConfig{
		Name:        part.Name,
		FrameConfig: frameConfig,
		ModelJson:   modelJSON,
	}, nil
}

// ProtobufToFrameSystemPart takes a protobuf object and transforms it into a FrameSystemPart
func ProtobufToFrameSystemPart(fsc *pb.FrameSystemConfig) (*FrameSystemPart, error) {
	if fsc == nil {
		return nil, nil
	}
	pose := spatialmath.NewPoseFromProtobuf(fsc.FrameConfig.Pose)
	point := pose.Point()
	translation := spatialmath.Translation{X: point.X, Y: point.Y, Z: point.Z}
	frameConfig := &Frame{
		Parent:      fsc.FrameConfig.Parent,
		Translation: translation,
		Orientation: pose.Orientation(),
	}
	modelFrame, err := referenceframe.ParseJSON(fsc.ModelJson, fsc.Name)
	if err != nil {
		return nil, err
	}
	return &FrameSystemPart{
		Name:        fsc.Name,
		FrameConfig: frameConfig,
		ModelFrame:  modelFrame,
	}, nil
}
