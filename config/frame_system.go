package config

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// Name is the robot part name, FrameConfig gives the general structure of the frame system,
// and ModelFrameConfig is an optional ModelJSON that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	Name        string
	FrameConfig *Frame
	ModelFrame  referenceframe.Model
}

// ToProtobuf turns all the interfaces into serializable types.
func (part *FrameSystemPart) ToProtobuf() (*pb.FrameSystemConfig, error) {
	if part.FrameConfig == nil {
		return nil, referenceframe.ErrNoModelInformation
	}
	poseInFrame := &commonpb.PoseInFrame{
		ReferenceFrame: part.FrameConfig.Parent,
		Pose:           spatialmath.PoseToProtobuf(part.FrameConfig.Pose()),
	}
	var modelJSON []byte
	var err error
	if part.ModelFrame != nil {
		modelJSON, err = part.ModelFrame.MarshalJSON()
		if err != nil {
			return nil, err
		}
	}
	return &pb.FrameSystemConfig{
		Name:              part.Name,
		PoseInParentFrame: poseInFrame,
		ModelJson:         modelJSON,
	}, nil
}

// ProtobufToFrameSystemPart takes a protobuf object and transforms it into a FrameSystemPart.
func ProtobufToFrameSystemPart(fsc *pb.FrameSystemConfig) (*FrameSystemPart, error) {
	pose := spatialmath.NewPoseFromProtobuf(fsc.PoseInParentFrame.Pose)
	frameConfig := &Frame{
		Parent:      fsc.PoseInParentFrame.ReferenceFrame,
		Translation: pose.Point(),
		Orientation: pose.Orientation(),
	}
	part := &FrameSystemPart{
		Name:        fsc.Name,
		FrameConfig: frameConfig,
	}
	modelFrame, err := referenceframe.UnmarshalModelJSON(fsc.ModelJson, fsc.Name)
	if err != nil {
		if errors.Is(err, referenceframe.ErrNoModelInformation) {
			return part, nil
		}
		return nil, err
	}
	part.ModelFrame = modelFrame
	return part, nil
}

// PoseInFrameToFrameSystemPart creates a FrameSystem part out of a PoseInFrame.
func PoseInFrameToFrameSystemPart(transform *referenceframe.PoseInFrame) (*FrameSystemPart, error) {
	if transform.Name() == "" || transform.FrameName() == "" {
		return nil, referenceframe.ErrEmptyStringFrameName
	}
	frameConfig := &Frame{
		Parent:      transform.FrameName(),
		Translation: transform.Pose().Point(),
		Orientation: transform.Pose().Orientation(),
	}
	part := &FrameSystemPart{
		Name:        transform.Name(),
		FrameConfig: frameConfig,
	}
	return part, nil
}

// CreateFramesFromPart will gather the frame information and build the frames from the given robot part.
func CreateFramesFromPart(part *FrameSystemPart, logger golog.Logger) (referenceframe.Frame, referenceframe.Frame, error) {
	if part == nil || part.FrameConfig == nil {
		return nil, nil, errors.New("config for FrameSystemPart is nil")
	}
	var modelFrame referenceframe.Frame
	var err error
	// use identity frame if no model frame defined
	if part.ModelFrame == nil {
		modelFrame = referenceframe.NewZeroStaticFrame(part.Name)
	} else {
		part.ModelFrame.ChangeName(part.Name)
		modelFrame = part.ModelFrame
	}
	// staticOriginFrame defines a change in origin from the parent part.
	// If it is empty, the new frame will have the same origin as the parent.
	staticOriginName := part.Name + "_origin"
	staticOriginFrame, err := part.FrameConfig.StaticFrame(staticOriginName)
	if err != nil {
		return nil, nil, err
	}
	return modelFrame, staticOriginFrame, nil
}
