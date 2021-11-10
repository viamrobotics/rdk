package framesystem

import (
	"context"
	"fmt"
	"sort"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/kinematics"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/utils"
)

// BuildFrameSystem uses a map of frames that describes the tree structure of the frame system to build a completed frame system
func BuildFrameSystem(ctx context.Context, name string, children map[string][]referenceframe.Frame, logger golog.Logger) (referenceframe.FrameSystem, error) {
	// If there are no frames, build an empty frame system with only a world node and return.
	if len(children) == 0 {
		return referenceframe.NewEmptySimpleFrameSystem(name), nil
	}
	// use a stack to populate the frame system
	stack := make([]string, 0)
	visited := make(map[string]bool)
	// check to see if world exists, and start with the frames attached to world
	if _, ok := children[referenceframe.World]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, referenceframe.World)
	// begin adding frames to the frame system
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, errors.Errorf("the system contains a cycle, have already visited frame %s", parent)
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
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(fs))
	return fs, nil
}

// CreateFramesFromPart will gather the frame information and build the frames from the given robot part
func CreateFramesFromPart(part *config.FrameSystemPart, logger golog.Logger) (referenceframe.Frame, referenceframe.Frame, error) {
	if part == nil {
		return nil, nil, errors.New("config for FrameSystemPart is nil")
	}
	var modelFrame referenceframe.Frame
	var err error
	// use identity frame if no model frame defined
	if part.ModelFrameConfig == nil {
		modelFrame = referenceframe.NewZeroStaticFrame(part.Name)
	} else {
		modelFrame, err = kinematics.ParseJSON(part.ModelFrameConfig, part.Name)
		if err != nil {
			return nil, nil, err
		}
	}
	// static frame defines an offset from the parent part-- if it is empty, a 0 offset frame will be applied.
	staticOffsetName := part.Name + "_offset"
	staticOffsetFrame, err := part.FrameConfig.StaticFrame(staticOffsetName)
	if err != nil {
		return nil, nil, err
	}
	return modelFrame, staticOffsetFrame, nil
}

// CollectFrameSystemParts collects the physical parts of the robot that may have frame info (excluding remote robots and services, etc)
// don't collect remote components, even though the Config lists them.
func CollectFrameSystemParts(ctx context.Context, r robot.Robot) (map[string]*config.FrameSystemPart, error) {
	parts := make(map[string]*config.FrameSystemPart)
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
		parts[c.Name] = &config.FrameSystemPart{Name: c.Name, FrameConfig: c.Frame, ModelFrameConfig: modelJSON}
	}
	return parts, nil
}

// processPart will gather the frame information and build the frames from the given robot part
func processPart(part *config.FrameSystemPart, children map[string][]referenceframe.Frame, names map[string]bool, logger golog.Logger) error {
	// if a part is empty or has no frame config, skip over it
	if part == nil || part.FrameConfig == nil {
		return nil
	}
	// parent field is a necessary attribute
	if part.FrameConfig.Parent == "" {
		return fmt.Errorf("parent field in frame config for part %q is empty", part.Name)
	}
	// build the frames from the part config
	modelFrame, staticOffsetFrame, err := CreateFramesFromPart(part, logger)
	if err != nil {
		return err
	}
	// check to see if there are no repeated names
	if ok := names[staticOffsetFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", staticOffsetFrame.Name())
	}
	names[staticOffsetFrame.Name()] = true
	if ok := names[modelFrame.Name()]; ok {
		return fmt.Errorf("cannot have more than one frame with name %s", modelFrame.Name())
	}
	names[modelFrame.Name()] = true
	// attach the static frame to the parent, then the model frame to the static frame
	children[part.FrameConfig.Parent] = append(children[part.FrameConfig.Parent], staticOffsetFrame)
	children[staticOffsetFrame.Name()] = append(children[staticOffsetFrame.Name()], modelFrame)
	return nil
}

func topologicallySortFrameNames(ctx context.Context, children map[string][]referenceframe.Frame) ([]string, error) {
	topoSortedNames := []string{referenceframe.World} // keep track of tree structure
	// If there are no frames, return only the world node in the list
	if len(children) == 0 {
		return topoSortedNames, nil
	}
	stack := make([]string, 0)
	visited := make(map[string]bool)
	if _, ok := children[referenceframe.World]; !ok {
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, referenceframe.World)
	// begin adding frames to the frame system
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		sort.Slice(children[parent], func(i, j int) bool {
			return children[parent][i].Name() < children[parent][j].Name()
		}) // sort alphabetically within the topological sort
		for _, frame := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, frame.Name())
			topoSortedNames = append(topoSortedNames, frame.Name())
		}
	}
	return topoSortedNames, nil
}

// ModelFramer has a method that returns the kinematics information needed to build a dynamic frame.
type ModelFramer interface {
	ModelFrame() []byte
}

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func extractModelFrameJSON(ctx context.Context, r robot.Robot, name string, compType config.ComponentType) ([]byte, error) {
	switch compType {
	case config.ComponentTypeBase:
		part, ok := r.BaseByName(name)
		if !ok {
			return nil, errors.Errorf("no base found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeGripper:
		part, ok := r.GripperByName(name)
		if !ok {
			return nil, errors.Errorf("no gripper found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeCamera:
		part, ok := r.CameraByName(name)
		if !ok {
			return nil, errors.Errorf("no camera found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeLidar:
		part, ok := r.LidarByName(name)
		if !ok {
			return nil, errors.Errorf("no lidar found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeSensor:
		part, ok := r.SensorByName(name)
		if !ok {
			return nil, errors.Errorf("no sensor found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeBoard:
		part, ok := r.BoardByName(name)
		if !ok {
			return nil, errors.Errorf("no board found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeServo:
		part, ok := r.ServoByName(name)
		if !ok {
			return nil, errors.Errorf("no servo found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeMotor:
		part, ok := r.MotorByName(name)
		if !ok {
			return nil, errors.Errorf("no motor found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	case config.ComponentTypeArm:
		part, ok := r.ResourceByName(arm.Named(name))
		if !ok {
			return nil, errors.Errorf("no resource found with name %q when extracting model frame json", name)
		}
		if framer, ok := utils.UnwrapProxy(part).(ModelFramer); ok {
			return framer.ModelFrame(), nil
		}
		return nil, nil
	default:
		return nil, errors.Errorf("do not recognize component type %v for model frame extraction", compType)
	}
}

func frameNamesWithDof(sys referenceframe.FrameSystem) []string {
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
