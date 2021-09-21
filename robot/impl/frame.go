package robotimpl

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/core/config"
	ref "go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/golang/geo/r3"
)

// CreateReferenceFrameSystem takes a robot and implements the FrameSystem api
func CreateReferenceFrameSystem(ctx context.Context, r robot.Robot) (ref.FrameSystem, error) {
	cfg, err := r.Config(ctx)
	if err != nil {
		return nil, err
	}

	// build each frame in the config file
	children := map[string][]ref.Frame{}
	names := map[string]bool{}
	names[ref.World] = true
	// loop through components and grab the components that have frame info
	for _, c := range cfg.Components {
		if c.Name == "" {
			return nil, errors.New("all components need names")
		}
		if c.Frame != nil {
			modelFrame, err := makeModelFrame(&c)
			if err != nil {
				return nil, err
			}
			staticName := c.Name + "_offset"
			// add the static frame first-- if it is empty, a 0 offset frame will be applied.
			staticFrame, err := makeStaticFrame(&c, staticName)
			if err != nil {
				return nil, err
			}
			// check to see if there are no repeated names
			if _, ok := names[staticFrame.Name()]; ok {
				return nil, fmt.Errorf("cannot have more than one frame with name %s", staticFrame.Name())
			}
			names[staticFrame.Name()] = true
			// attach the static frame to the parent
			children[c.Frame.Parent] = append(children[c.Frame.Parent], staticFrame)

			// if the model frame exists, add it as well
			if _, ok := names[modelFrame.Name()]; ok {
				return nil, fmt.Errorf("cannot have more than one frame with name %s", modelFrame.Name())
			}
			names[modelFrame.Name()] = true
			// store the children Frames in a list to build the tree later
			children[staticFrame.Name()] = append(children[staticFrame.Name()], modelFrame)
		}
	}
	return buildFrameSystem("robot", names, children)
}

func makePoseFromConfig(f *config.FrameConfig) spatial.Pose {
	// get the translation vector. If there is no translation/orientation attribute will default to 0
	translation := r3.Vector{f.Translation.X, f.Translation.Y, f.Translation.Z}

	ov := &spatial.OrientationVec{utils.DegToRad(f.Orientation.TH), f.Orientation.X, f.Orientation.Y, f.Orientation.Z}
	return spatial.NewPoseFromOrientationVector(translation, ov)
}

func makeStaticFrame(comp *config.Component, name string) (ref.Frame, error) {
	pose := makePoseFromConfig(comp.Frame)
	return ref.NewStaticFrame(name, pose)
}

func makeModelFrame(comp *config.Component) (ref.Frame, error) {
	if frameFunc, ok := registry.FrameLookup(comp); ok {
		return frameFunc(comp.Name)
	}
	// return identity frame if no frame function
	return ref.NewZeroStaticFrame(comp.Name), nil
}

func buildFrameSystem(name string, frameNames map[string]bool, children map[string][]ref.Frame) (ref.FrameSystem, error) {
	// use a stack to populate the frame system
	stack := make([]string, 0)
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children[ref.World]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, ref.World)
	// begin adding frames to the frame system
	fs := ref.NewEmptySimpleFrameSystem(name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
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
