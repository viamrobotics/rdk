package framesystem

import (
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/utils"
)

type Parts []*referenceframe.FrameSystemPart

// Config is a slice of *config.FrameSystemPart.
type Config struct {
	resource.TriviallyValidateConfig
	Parts
	AdditionalTransforms []*referenceframe.LinkInFrame
}

// String prints out a table of each frame in the system, with columns of name, parent, translation and orientation.
func (cfg Config) String() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation", "Geometry"})
	t.AppendRow([]interface{}{"0", referenceframe.World, "", "", "", ""})
	for i, part := range cfg.Parts {
		pose := part.FrameConfig.Pose()
		tra := pose.Point()
		ori := pose.Orientation().EulerAngles()
		geomString := ""
		if gc := part.FrameConfig.Geometry(); gc != nil {
			geomString = gc.String()
		}
		t.AppendRow([]interface{}{
			fmt.Sprintf("%d", i+1),
			part.FrameConfig.Name(),
			part.FrameConfig.Parent(),
			fmt.Sprintf("X:%.0f, Y:%.0f, Z:%.0f", tra.X, tra.Y, tra.Z),
			fmt.Sprintf(
				"Roll:%.2f, Pitch:%.2f, Yaw:%.2f",
				utils.RadToDeg(ori.Roll),
				utils.RadToDeg(ori.Pitch),
				utils.RadToDeg(ori.Yaw),
			),
			geomString,
		})
	}
	return t.Render()
}

// Prefixs applies prefixes to frame information if necessary.
func PrefixRemoteParts(parts Parts, remoteName string, remoteParent string) {
	for _, part := range parts {
		if part.FrameConfig.Parent() == referenceframe.World { // rename World of remote parts
			part.FrameConfig.SetParent(remoteParent)
		}
		// rename each non-world part with prefix
		part.FrameConfig.SetName(remoteName + ":" + part.FrameConfig.Name())
		if part.FrameConfig.Parent() != remoteParent {
			part.FrameConfig.SetParent(remoteName + ":" + part.FrameConfig.Parent())
		}
	}
}

// Names returns the names of input parts.
func Names(parts Parts) []string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.FrameConfig.Name()
	}
	return names
}

// NewFrameSystemFromConfig assembles a frame system from a given config.
func NewFrameSystemFromConfig(name string, cfg *Config) (referenceframe.FrameSystem, error) {
	allParts := make(Parts, 0)
	allParts = append(allParts, cfg.Parts...)
	for _, tf := range cfg.AdditionalTransforms {
		transformPart, err := referenceframe.LinkInFrameToFrameSystemPart(tf)
		if err != nil {
			return nil, err
		}
		allParts = append(allParts, transformPart)
	}

	// ensure that at least one frame connects to world if the frame system is not empty
	if len(allParts) != 0 {
		hasWorld := false
		for _, part := range allParts {
			if part.FrameConfig.Parent() == referenceframe.World {
				hasWorld = true
				break
			}
		}
		if !hasWorld {
			return nil, errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
		}
	}
	// Topologically sort parts
	sortedParts, err := TopologicallySort(allParts)
	if err != nil {
		return nil, err
	}
	if len(sortedParts) != len(allParts) {
		return nil, errors.Errorf(
			"frame system has disconnected frames. connected frames: %v, all frames: %v",
			Names(sortedParts),
			Names(allParts),
		)
	}
	fs := referenceframe.NewEmptySimpleFrameSystem(name)
	for _, part := range sortedParts {
		// rename everything with prefixes
		part.FrameConfig.SetName(part.FrameConfig.Name())
		// prefixing for the world frame is only necessary in the case
		// of merging multiple frame systems together, so we leave that
		// reponsibility to the corresponding merge function
		if part.FrameConfig.Parent() != referenceframe.World {
			part.FrameConfig.SetParent(part.FrameConfig.Parent())
		}
		// make the frames from the configs
		modelFrame, staticOffsetFrame, err := referenceframe.CreateFramesFromPart(part)
		if err != nil {
			return nil, err
		}
		// attach static offset frame to parent, attach model frame to static offset frame
		err = fs.AddFrame(staticOffsetFrame, fs.Frame(part.FrameConfig.Parent()))
		if err != nil {
			return nil, err
		}
		err = fs.AddFrame(modelFrame, staticOffsetFrame)
		if err != nil {
			return nil, err
		}
	}
	return fs, nil
}

// TopologicallySort takes a potentially un-ordered slice of frame system parts and
// sorts them, beginning at the world node.
func TopologicallySort(parts Parts) (Parts, error) {
	// set up directory to check existence of parents
	existingParts := make(map[string]bool, len(parts))
	existingParts[referenceframe.World] = true
	for _, part := range parts {
		existingParts[part.FrameConfig.Name()] = true
	}
	// make map of children
	children := make(map[string]Parts)
	for _, part := range parts {
		parent := part.FrameConfig.Parent()
		if !existingParts[parent] {
			return nil, NewMissingParentError(part.FrameConfig.Name(), parent)
		}
		children[part.FrameConfig.Parent()] = append(children[part.FrameConfig.Parent()], part)
	}
	topoSortedParts := Parts{} // keep track of tree structure
	// If there are no frames, return the empty list
	if len(children) == 0 {
		return topoSortedParts, nil
	}
	stack := make([]string, 0)
	visited := make(map[string]bool)
	if _, ok := children[referenceframe.World]; !ok {
		return nil, errors.New("there are no robot parts that connect to a 'world' node. Root node must be named 'world'")
	}
	stack = append(stack, referenceframe.World)
	// begin adding frames to tree
	for len(stack) != 0 {
		parent := stack[0] // pop the top element from the stack
		stack = stack[1:]
		if _, ok := visited[parent]; ok {
			return nil, fmt.Errorf("the system contains a cycle, have already visited frame %s", parent)
		}
		visited[parent] = true
		sort.Slice(children[parent], func(i, j int) bool {
			return children[parent][i].FrameConfig.Name() < children[parent][j].FrameConfig.Name()
		}) // sort alphabetically within the topological sort
		for _, part := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, part.FrameConfig.Name())
			topoSortedParts = append(topoSortedParts, part)
		}
	}
	return topoSortedParts, nil
}
