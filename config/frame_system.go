package config

import (
	"encoding/json"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/robot/v1"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// Name is the robot part name, FrameConfig gives the general structure of the frame system,
// and ModelFrameConfig is an optional ModelJSON that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	FrameConfig *referenceframe.LinkCfg
	ModelFrame  referenceframe.Model
}

// ToProtobuf turns all the interfaces into serializable types.
func (part *FrameSystemPart) ToProtobuf() (*pb.FrameSystemConfig, error) {
	if part.FrameConfig == nil {
		return nil, referenceframe.ErrNoModelInformation
	}
	pose, err := part.FrameConfig.Pose()
	if err != nil {
		return nil, err
	}
	
	geom, err := part.FrameConfig.Geometry.ToProtobuf()
	if err != nil {
		return nil, err
	}
	linkFrame := &commonpb.StaticFrame{
		Name: part.FrameConfig.ID,
		PoseInParentFrame: &commonpb.PoseInFrame{
			ReferenceFrame: part.FrameConfig.Parent,
			Pose:           spatialmath.PoseToProtobuf(pose),
		},
		Geometries: []*commonpb.Geometry{geom},
	}
	var modelJson map[string]interface{}
	if part.ModelFrame != nil {
		bytes, err := part.ModelFrame.MarshalJSON()
		if err != nil {
			return nil, err
		}
		json.Unmarshal(bytes, &modelJson)
	}
	kinematics, err := protoutils.StructToStructPb(modelJson)
	if err != nil {
		return nil, err
	}
	return &pb.FrameSystemConfig{
		Frame: linkFrame,
		Kinematics:         kinematics,
	}, nil
}

// ProtobufToFrameSystemPart takes a protobuf object and transforms it into a FrameSystemPart.
func ProtobufToFrameSystemPart(fsc *pb.FrameSystemConfig) (*FrameSystemPart, error) {
	pose := spatialmath.NewPoseFromProtobuf(fsc.Frame.PoseInParentFrame.Pose)
	orient, err := spatialmath.NewOrientationConfig(pose.Orientation())
	if err != nil {
		return nil, err
	}
	var geom *spatialmath.GeometryConfig
	if len(fsc.Frame.Geometries) > 0 {
		geomTemp, err := spatialmath.NewGeometryCreatorFromProto(fsc.Frame.Geometries[0])
		if err != nil {
			return nil, err
		}
		geom, err = spatialmath.NewGeometryConfig(geomTemp)
		if err != nil {
			return nil, err
		}
	}

	frameConfig := &referenceframe.LinkCfg{
		&referenceframe.StaticFrameCfg{
			ID: fsc.Frame.Name,
			Translation: *spatialmath.NewTranslationConfig(pose.Point()),
			Orientation: orient,
			Geometry: geom,
		},
		fsc.Frame.PoseInParentFrame.ReferenceFrame,
	}
	part := &FrameSystemPart{
		FrameConfig: frameConfig,
	}
	fmt.Println("id", frameConfig.ID)
	fmt.Println("fsc", fsc.Frame)
	fmt.Println("part", part.FrameConfig)
	if len(fsc.Kinematics.AsMap()) > 0 {
		modelBytes, err := json.Marshal(fsc.Kinematics.AsMap())
		if err != nil {
			return nil, err
		}
		modelFrame, err := referenceframe.UnmarshalModelJSON(modelBytes, fsc.Frame.Name)
		if err != nil {
			if errors.Is(err, referenceframe.ErrNoModelInformation) {
				return part, nil
			}
			return nil, err
		}
		part.ModelFrame = modelFrame
	}
	return part, nil
}

// PoseInFrameToFrameSystemPart creates a FrameSystem part out of a PoseInFrame.
func PoseInFrameToFrameSystemPart(transform *referenceframe.PoseInFrame) (*FrameSystemPart, error) {
	if transform.Name() == "" || transform.FrameName() == "" {
		return nil, referenceframe.ErrEmptyStringFrameName
	}
	orient, err := spatialmath.NewOrientationConfig(transform.Pose().Orientation())
	if err != nil {
		return nil, err
	}
	frameConfig := &referenceframe.LinkCfg{
		StaticFrameCfg: &referenceframe.StaticFrameCfg{
			ID:      transform.Name(),
			Translation: *spatialmath.NewTranslationConfig(transform.Pose().Point()),
			Orientation: orient,
		},
		Parent: transform.FrameName(),
	}
	part := &FrameSystemPart{
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
		modelFrame = referenceframe.NewZeroStaticFrame(part.FrameConfig.ID)
	} else {
		part.ModelFrame.ChangeName(part.FrameConfig.ID)
		modelFrame = part.ModelFrame
	}
	// staticOriginFrame defines a change in origin from the parent part.
	// If it is empty, the new frame will have the same origin as the parent.
	staticOriginName := part.FrameConfig.ID + "_origin"
	staticOriginFrame, err := part.FrameConfig.ToStaticFrame(staticOriginName)
	if err != nil {
		return nil, nil, err
	}
	return modelFrame, staticOriginFrame, nil
}
