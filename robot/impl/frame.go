package robotimpl

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/core/config"
	ref "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
)

// CreateReferenceFrameSystem takes a robot and implements the FrameSystem api
func CreateReferenceFrameSystem(ctx context.Context, r robot.Robot) (ref.FrameSystem, error) {
	cfg, err := r.Config(ctx)
	if err != nil {
		return nil, err
	}

	// build each frame in the config file
	children := make(map[string][]ref.Frame)
	names := make(map[string]bool)
	names["world"] = true
	// loop through components and grab the components that have frame info
	for _, c := range cfg.Components {
		if c.Name == "" {
			return nil, errors.New("all components need names")
		}
		if c.Frame != nil {
			var frame ref.Frame
			var err error
			switch c.Frame.Type {
			case config.FrameTypeStatic:
				frame, err = makeStaticFrame(&c)
			case config.FrameTypeModel:
				frame, err = makeModelFrame(&c)
			default:
				return nil, fmt.Errorf("do not know how to create Frame of type %s", string(c.Frame.Type))
			}
			if err != nil {
				return nil, err
			}
			if frame == nil {
				return nil, errors.New("frame is nil")
			}
			// check to see if there are no repeated names
			if _, ok := names[frame.Name()]; ok {
				return nil, fmt.Errorf("cannot have more than one Frame with name %s", frame.Name())
			}
			names[frame.Name()] = true
			// store the children Frames in a list to build the tree later
			children[c.Frame.Parent] = append(children[c.Frame.Parent], frame)
		}
	}
	return buildFrameSystem("robot", names, children)
}

func makeStaticFrame(comp *config.Component) (ref.Frame, error) {
	f := comp.Frame
	// get the translation vector
	translation := r3.Vector{f.Translate.X, f.Translate.Y, f.Translate.Z}

	var pose spatial.Pose
	// get the orientation if there is one
	if f.SetOrientation {
		ov := &spatial.OrientationVec{f.Orient.TH, f.Orient.X, f.Orient.Y, f.Orient.Z}
		pose = spatial.NewPoseFromOrientationVector(translation, ov)
	} else {
		pose = spatial.NewPoseFromPoint(translation)
	}
	// create and set the frame
	return ref.NewStaticFrame(comp.Name, pose), nil
}

func makeModelFrame(comp *config.Component) (ref.Frame, error) {
	var modelFrame ref.Frame
	var err error
	// get the frame as registered in the component model
	switch comp.Type {
	case config.ComponentTypeProvider:
		registration := registry.ProviderLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeBase:
		registration := registry.BaseLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeArm:
		registration := registry.ArmLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeGripper:
		registration := registry.GripperLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeCamera:
		registration := registry.CameraLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeLidar:
		registration := registry.LidarLookup(comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	case config.ComponentTypeSensor:
		if comp.SubType == "" {
			return nil, errors.New("sensor component requires subtype")
		}
		registration := registry.SensorLookup(sensor.Type(comp.SubType), comp.Model)
		if registration == nil && registration.Frame == nil {
			return nil, errors.New("component has nil for Frame")
		}
		modelFrame, err = registration.Frame()
	default:
		return nil, fmt.Errorf("unknown component type: %v", comp.Type)
	}
	if err != nil {
		return nil, err
	}
	return modelFrame, nil
}

func buildFrameSystem(name string, frameNames map[string]bool, children map[string][]ref.Frame) (ref.FrameSystem, error) {
	// use a stack to populate the frame system
	stack := make([]string, 0)
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children["world"]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'.")
	}
	stack = append(stack, "world")
	// begin adding frames to the frame system
	fs := ref.NewEmptySimpleFrameSystem(name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		} else {
			visited[parent] = true
		}
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			err := fs.AddFrame(frame, fs.GetFrame(parent))
			if err != nil {
				return nil, err
			}
		}
	}
	// ensure that there are no disconnected frames
	if len(visited) != len(frameNames) {
		return nil, fmt.Errorf("the system is not fully connected, expected %d frames but frame system has %d", len(frameNames), len(visited))
	}
	return fs, nil
}
