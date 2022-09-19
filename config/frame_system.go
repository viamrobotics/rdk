package config

import (
	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/robot/v1"
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
	pose := spatialmath.PoseToProtobuf(part.FrameConfig.Pose())
	convertedPose := &commonpb.Pose{
		X:     pose.X,
		Y:     pose.Y,
		Z:     pose.Z,
		OX:    pose.OX,
		OY:    pose.OY,
		OZ:    pose.OZ,
		Theta: pose.Theta,
	}
	poseInFrame := &commonpb.PoseInFrame{
		ReferenceFrame: part.FrameConfig.Parent,
		Pose:           convertedPose,
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
	poseMsg := fsc.PoseInParentFrame.Pose
	convertedPose := &commonpb.Pose{
		X:     poseMsg.X,
		Y:     poseMsg.Y,
		Z:     poseMsg.Z,
		OX:    poseMsg.OX,
		OY:    poseMsg.OY,
		OZ:    poseMsg.OZ,
		Theta: poseMsg.Theta,
	}
	pose := spatialmath.NewPoseFromProtobuf(convertedPose)
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

// NewMissingReferenceFrameError returns an error indicating that a particular
// protobuf message is missing necessary information for its ReferenceFrame key.
func NewMissingReferenceFrameError(msg interface{}) error {
	return errors.Errorf("missing reference frame in protobuf message of type %T", msg)
}

// ConvertTransformProtobufToFrameSystemPart creates a FrameSystem part out of a
// transform protobuf message.
func ConvertTransformProtobufToFrameSystemPart(transformMsg *commonpb.Transform) (*FrameSystemPart, error) {
	frameName := transformMsg.GetReferenceFrame()
	if frameName == "" {
		return nil, NewMissingReferenceFrameError(transformMsg)
	}
	poseInObserverFrame := transformMsg.GetPoseInObserverFrame()
	parentFrame := poseInObserverFrame.GetReferenceFrame()
	if parentFrame == "" {
		return nil, NewMissingReferenceFrameError(poseInObserverFrame)
	}
	poseMsg := poseInObserverFrame.GetPose()
	pose := spatialmath.NewPoseFromProtobuf(poseMsg)
	frameConfig := &Frame{
		Parent:      parentFrame,
		Translation: pose.Point(),
		Orientation: pose.Orientation(),
	}
	part := &FrameSystemPart{
		Name:        frameName,
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
