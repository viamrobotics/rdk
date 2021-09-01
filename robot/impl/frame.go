package robotimpl

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	ref "go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
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
			switch c.Frame.Type {
			case config.FrameTypeStatic:
				frame = makeStaticFrame(&c)
			case config.FrameTypePrismatic:
				frame = makePrismaticFrame(&c)
			case config.FrameTypeRevolute:
				frame = makeRevoluteFrame(&c)
			case config.FrameTypeModel:
				frame = makeModelFrame(&c)
			default:
				return nil, fmt.Errorf("do not know how to create Frame of type %s", string(c.Frame.Type))
			}
			// check to see if there are no repeated names
			if _, ok := names[frame.Name()]; ok {
				return nil, fmt.Errorf("cannot have more than one Frame with name %s", frame.Name())
			}
			names[frame.Name()] = true
			// store the children Frames in a list to build the tree later
			children[f.Parent] = append(children[f.Parent], frame)
		}
	}
	return buildFrameSystem("robot", names, children)
}

func makeStaticFrame(comp *config.Component) ref.Frame {
	var pose spatial.Pose
	f := comp.Frame
	// get the translation vector
	translation := r3.Vector{f.Translation.X, f.Translation.Y, f.Translation.Z}

	// get the orientation if there is one
	if f.SetOrientation {
		ov := &spatial.OrientationVec{f.Orientation.T, f.Orientation.X, f.Orientation.Y, f.Orientation.Z}
		pose = spatial.NewPoseFromOrientationVector(translation, ov)
	} else {
		pose = spatial.NewPoseFromPoint(translation)
	}
	// create and set the frame
	return ref.NewStaticFrame(comp.Name, pose)
}

func makePrismaticFrame(comp *config.Component) ref.Frame {
	// get the translation axes
	f := comp.Frame
	axes := []bool{f.Axes.X, f.Axes.Y, f.Axes.Z}

	// create and set the frame
	prism := ref.NewPrismaticFrame(comp.Name, axes)
	prism.SetLimits(f.Min, f.Max)
	return prism
}

func makeRevoluteFrame(comp *config.Component) ref.Frame {
	f := comp.Frame
	// get the rotation axis
	axis := spatial.R4AA{RX: f.Axis.X, RY: f.Axis.Y, RZ: f.Axis.Z}

	// create and set the frame
	rev := ref.NewRevoluteFrame(comp.Name, axis)
	rev.SetLimits(f.Min[0], f.Max[0])
	return rev
}

func makeModelFrame(comp *config.Component) ref.Frame {
	// get the frame model from the kinematics model
	// if there is a static offset, add it
	// create and set the frame
	return mf
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
