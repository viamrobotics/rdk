package config

import (
	"context"

	"go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/go-errors/errors"
)

// FrameSystemPart is used to collect all the info need from a named robot part to build the frame node in a frame system.
// Name is the robot part name, FrameConfig gives the general structure of the frame system,
// and ModelFrameConfig is an optional JSON btye stream that describes the internal kinematics of the robot part.
type FrameSystemPart struct {
	Name             string
	FrameConfig      *Frame
	ModelFrameConfig []byte
}

// FrameSystemPartToProtobuf turns all the interfaces into serializable types
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
	pose := spatialmath.NewPoseFromProtobuf(fsc.FrameConfig.Pose)
	return &FrameSystemPart{
		Name: fsc.Name,
		FrameConfig: &Frame{
			Parent:      fsc.FrameConfig.Parent,
			Translation: Translation{X: pose.Point().X, pose.Point().Y, pose.Point().Z},
			Orientation: pose.Orientation(),
		},
		ModelFrameConfig: fsc.ModelJson,
	}
}

// CreateFramesFromPart will gather the frame information and build the frames from the given robot part
func CreateFramesFromPart(part *FrameSystemPart) (referenceframe.Frame, referenceframe.Frame, error) {
	if part == nil {
		return nil, nil, errors.New("config for FrameSystemPart is nil")
	}
	var modelFrame referenceframe.Frame
	// use identity frame if no model frame defined
	if part.ModelFrameConfig == nil {
		modelFrame = referenceframe.NewZeroStaticFrame(part.Name)
	} else {
		model, err := kinematics.ParseJSON(part.ModelFrameConfig)
		if err != nil {
			return nil, nil, err
		}
		modelFrame = model.Clone(part.Name)
	}
	// static frame defines an offset from the parent part-- if it is empty, a 0 offset frame will be applied.
	staticOffsetName := part.Name + "_offset"
	staticOffsetFrame, err := MakeStaticFrame(part.FrameConfig, staticOffsetName)
	if err != nil {
		return nil, nil, err
	}
	return modelFrame, staticOffsetFrame, nil
}

// CollectFrameSystemParts collects the physical parts of the robot that may have frame info (excluding remote robots and services, etc)
// don't collect remote components, even though the Config lists them.
func CollectFrameSystemParts(ctx context.Context, r robot.Robot) (map[string]*FrameSystemPart, error) {
	logger := r.Logger()
	parts := make(map[string]*FrameSystemPart)
	seen := make(map[string]bool)
	cfg, err := r.Config(ctx) // Eventually there will be another function that gathers the frame system config
	if err != nil {
		return nil, err
	}
	for _, c := range cfg.Components {
		if c.Frame == nil || c.Model == "" { // no Frame means dont include in frame system. No Model means it's a remote part.
			continue
		}
		if _, ok := seen[c.Name]; ok {
			return nil, errors.Errorf("more than one component with name %q in config file", c.Name)
		}
		seen[c.Name] = true
		modelJSON, err := extractModelFrameJSON(ctx, r, c.Name, c.Type)
		if err != nil {
			return nil, err
		}
		parts[c.Name] = &FrameSystemPart{Name: c.Name, FrameConfig: c.Frame, ModelFrameConfig: modelJSON}
	}
	return parts
}

// ModelFramer has a method that returns the kinematics information needed to build a dynamic frame.
type ModelFramer interface {
	ModelFrame() []byte
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func extractModelFrameJSON(ctx context.Context, r robot.Robot, name string, compType ComponentType) ([]byte, error) {
	switch compType {
	case ComponentTypeBase:
		part, ok := r.BaseByName(name)
		if !ok {
			return nil, errors.Errorf("no base found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeGripper:
		part, ok := r.GripperByName(name)
		if !ok {
			return nil, errors.Errorf("no gripper found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeCamera:
		part, ok := r.CameraByName(name)
		if !ok {
			return nil, errors.Errorf("no camera found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeLidar:
		part, ok := r.LidarByName(name)
		if !ok {
			return nil, errors.Errorf("no lidar found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeSensor:
		part, ok := r.SensorByName(name)
		if !ok {
			return nil, errors.Errorf("no sensor found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeBoard:
		part, ok := r.BoardByName(name)
		if !ok {
			return nil, errors.Errorf("no board found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeServo:
		part, ok := r.ServoByName(name)
		if !ok {
			return nil, errors.Errorf("no servo found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeMotor:
		part, ok := r.MotorByName(name)
		if !ok {
			return nil, errors.Errorf("no motor found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	case ComponentTypeArm:
		part, ok := r.ResourceByName(name)
		if !ok {
			return nil, errors.Errorf("no resource found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		} else {
			return nil, nil
		}
	default:
		return nil, errors.Errorf("do not recognize component type %v for model frame extraction", compType)
	}
}
