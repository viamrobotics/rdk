// Package framesystem defines and implements the concept of a frame system.
package framesystem

import (
	"context"
	"fmt"
	"sort"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// NewFrameSystemFromParts assembles a frame system from a collection of parts,
// usually acquired by calling Config on a frame system service. WARNING: for now,
// this function requires that the parts are already topologically sorted (see
// topologicallySortFrameNames below for a loose example of that process).
func NewFrameSystemFromParts(
	name, prefix string, parts []*config.FrameSystemPart,
	logger golog.Logger,
) (referenceframe.FrameSystem, error) {
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for _, part := range parts {
		// rename everything with prefixes
		part.Name = prefix + part.Name
		// prefixing for the world frame is only necessary in the case
		// of merging multiple frame systems together, so we leave that
		// reponsibility to the corresponding merge function
		if part.FrameConfig.Parent != referenceframe.World {
			part.FrameConfig.Parent = prefix + part.FrameConfig.Parent
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, logger)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.GetFrame(part.FrameConfig.Parent))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	logger.Debugf("frames in robot frame system are: %v", frameNamesWithDof(fs))
	return fs, nil
}

// CollectFrameSystemParts collects the physical parts of the robot that may have frame info (excluding remote robots and services, etc)
// don't collect remote components.
func CollectFrameSystemParts(ctx context.Context, r robot.Robot) (map[string]*config.FrameSystemPart, error) {
	parts := make(map[string]*config.FrameSystemPart)
	seen := make(map[string]bool)
	local, ok := r.(robot.LocalRobot)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("robot.LocalRobot", r)
	}
	cfg, err := local.Config(ctx) // Eventually there will be another function that gathers the frame system config
	if err != nil {
		return nil, err
	}
	for _, c := range cfg.Components {
		if c.Frame == nil { // no Frame means dont include in frame system.
			continue
		}
		if _, ok := seen[c.Name]; ok {
			return nil, errors.Errorf("more than one component with name %q in config file", c.Name)
		}
		seen[c.Name] = true
		model, err := extractModelFrameJSON(r, c.ResourceName())
		if err != nil && !errors.Is(err, referenceframe.ErrNoModelInformation) {
			return nil, err
		}
		parts[c.Name] = &config.FrameSystemPart{Name: c.Name, FrameConfig: c.Frame, ModelFrame: model}
	}
	return parts, nil
}

// processPart will gather the frame information and build the frames from the given robot part.
func processPart(
	part *config.FrameSystemPart,
	children map[string][]referenceframe.Frame,
	names map[string]bool,
	logger golog.Logger,
) error {
	// if a part is empty or has no frame config, skip over it
	if part == nil || part.FrameConfig == nil {
		return nil
	}
	// parent field is a necessary attribute
	if part.FrameConfig.Parent == "" {
		return fmt.Errorf("parent field in frame config for part %q is empty", part.Name)
	}
	// build the frames from the part config
	modelFrame, staticOffsetFrame, err := config.CreateFramesFromPart(part, logger)
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

func topologicallySortFrameNames(children map[string][]referenceframe.Frame) ([]string, error) {
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

// extractModelFrameJSON finds the robot part with a given name, checks to see if it implements ModelFrame, and returns the
// JSON []byte if it does, or nil if it doesn't.
func extractModelFrameJSON(r robot.Robot, name resource.Name) (referenceframe.Model, error) {
	part, err := r.ResourceByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "no resource found with name %q when extracting model frame json", name)
	}
	if framer, ok := utils.UnwrapProxy(part).(referenceframe.ModelFramer); ok {
		return framer.ModelFrame(), nil
	}
	return nil, referenceframe.ErrNoModelInformation
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
