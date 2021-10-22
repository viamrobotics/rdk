package robotimpl

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.viam.com/core/config"
	ref "go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

// FrameSystemLinker has a method that returns all the information needed to add the component
// to a FrameSystem.
type FrameSystemLinker interface {
	FrameSystemLink() (*config.Frame, ref.Frame)
}

// namedPart is used to collect the various named robot parts that could potentially have frame information
type namedPart struct {
	Name string
	Part interface{}
}

func CreateRobotFrameSystem(ctx context.Context, r robot.Robot, robotName string) (ref.FrameSystem, error) {
	// collect the necessary robot parts (skipping remotes, services, etc)
	parts := collectRobotParts(r)

	// collect the frame info from each part that will be used to build the system
	children := map[string][]ref.Frame{}
	names := map[string]bool{}
	names[ref.World] = true
	for _, part := range parts {
		if part.Name == "" {
			return nil, errors.New("part name cannot be empty")
		}
		err := createFrameFromPart(part, children, names)
		if err != nil {
			return nil, err
		}
	}
	return buildFrameSystem(robotName, names, children, r.Logger())
}

// MergeFrameSystemFromConfig will merge fromFS into toFS with an offset frame given by cfg. If cfg is nil, fromFS
// will be merged to the world frame of toFS with a 0 offset.
func MergeFrameSystemsFromConfig(toFS, fromFS ref.FrameSystem, cfg *config.Frame) error {
	var offsetFrame ref.Frame
	var err error
	if cfg == nil { // if nil, the parent is toFS's world, and the offset is 0
		offsetFrame = ref.NewZeroStaticFrame(fromFS.Name() + "_" + ref.World)
		err = toFS.AddFrame(offsetFrame, toFS.World())
		if err != nil {
			return err
		}
	} else { // attach the world of fromFS, with the given offset, to cfg.Parent found in toFS
		offsetFrame, err = makeStaticFrameFromConfig(cfg, fromFS.Name()+"_"+ref.World)
		if err != nil {
			return err
		}
		err = toFS.AddFrame(offsetFrame, toFS.GetFrame(cfg.Parent))
		if err != nil {
			return err
		}
	}
	err = toFS.MergeFrameSystem(fromFS, offsetFrame)
	if err != nil {
		return err
	}
	return nil
}

// collectRobotParts collects the physical parts of the robot that may have frame info (excluding remote robots and services, etc)
func collectRobotParts(r robot.Robot) []namedPart {
	parts := []namedPart{}
	for _, name := range r.BaseNames() {
		part, ok := r.BaseByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.BoardNames() {
		part, ok := r.BoardByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.CameraNames() {
		part, ok := r.CameraByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.GripperNames() {
		part, ok := r.GripperByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.LidarNames() {
		part, ok := r.LidarByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.SensorNames() {
		part, ok := r.SensorByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.ServoNames() {
		part, ok := r.ServoByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}
	for _, name := range r.MotorNames() {
		part, ok := r.MotorByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name, utils.UnwrapProxy(part)})
	}

	for _, name := range r.ResourceNames() {
		part, ok := r.ResourceByName(name)
		if !ok {
			continue
		}
		parts = append(parts, namedPart{name.Name, utils.UnwrapProxy(part)})
	}
	return parts
}

// createFrameFromPart will gather the frame information and build the frames from robot parts that have FrameSystemLink() defined.
func createFrameFromPart(part namedPart, children map[string][]ref.Frame, names map[string]bool) error {
	var modelFrame ref.Frame
	var frameConfig *config.Frame
	// part must have FrameSystemLink() defined to be added to a FrameSystem
	if fsl, ok := part.Part.(FrameSystemLinker); ok {
		frameConfig, modelFrame = fsl.FrameSystemLink()
	} else {
		return fmt.Errorf("part %q of type %T does not have FrameSystemLink() defined", part.Name, part.Part)
	}
	// if a part has no frame config, skip over it
	if frameConfig == nil {
		return nil
	}
	// parent field is a necessary attribute
	if frameConfig.Parent == "" {
		return fmt.Errorf("parent field in frame config for part %q is empty", part.Name)
	}
	// use identity frame if no model frame defined
	if modelFrame == nil {
		modelFrame = ref.NewZeroStaticFrame(part.Name)
	}
	// static frame defines an offset from the parent part-- if it is empty, a 0 offset frame will be applied.
	staticName := part.Name + "_offset"
	staticFrame, err := makeStaticFrameFromConfig(frameConfig, staticName)
	if err != nil {
		return err
	}
	// check to see if there are no repeated names
	if _, ok := names[staticFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", staticFrame.Name())
	}
	names[staticFrame.Name()] = true
	if _, ok := names[modelFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", modelFrame.Name())
	}
	names[modelFrame.Name()] = true
	// attach the static frame to the parent, then the model frame to the static frame
	children[frameConfig.Parent] = append(children[frameConfig.Parent], staticFrame)
	children[staticFrame.Name()] = append(children[staticFrame.Name()], modelFrame)
	return nil
}

func buildFrameSystem(name string, frameNames map[string]bool, children map[string][]ref.Frame, logger golog.Logger) (ref.FrameSystem, error) {
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
		return nil, fmt.Errorf("the frame system is not fully connected, expected %d frames but frame system has %d. Expected frames are: %v. Actual frames are: %v", len(frameNames), len(visited), mapKeys(frameNames), mapKeys(visited))
	}
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(fs))
	return fs, nil
}

func makeStaticFrameFromConfig(comp *config.Frame, name string) (ref.Frame, error) {
	pose := makePoseFromConfig(comp)
	return ref.NewStaticFrame(name, pose)
}

func makePoseFromConfig(f *config.Frame) spatial.Pose {
	// get the translation vector. If there is no translation/orientation attribute will default to 0
	translation := r3.Vector{f.Translation.X, f.Translation.Y, f.Translation.Z}
	return spatial.NewPoseFromOrientation(translation, f.Orientation)
}

func frameNamesWithDof(sys ref.FrameSystem) []string {
	names := sys.FrameNames()
	nameDoFs := make([]string, len(names))
	for i, f := range names {
		fr := sys.GetFrame(f)
		nameDoFs[i] = fmt.Sprintf("%s(%d)", fr.Name(), len(fr.DoF()))
	}
	return nameDoFs
}

func mapKeys(fullmap map[string]bool) []string {
	keys := make([]string, len(fullmap))
	i := 0
	for k := range fullmap {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}
