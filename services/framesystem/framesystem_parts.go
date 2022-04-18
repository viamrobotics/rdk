package framesystem

import (
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Parts is a slice of *config.FrameSystemPart.
type Parts []*config.FrameSystemPart

// String prints out a table of each frame in the system, with columns of name, parent, translation and orientation.
func (fsp Parts) String() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation"})
	t.AppendRow([]interface{}{"0", referenceframe.World, "", "", ""})
	for i, part := range fsp {
		tra := part.FrameConfig.Translation
		ori := &spatialmath.EulerAngles{}
		if part.FrameConfig.Orientation != nil {
			ori = part.FrameConfig.Orientation.EulerAngles()
		}
		t.AppendRow([]interface{}{
			fmt.Sprintf("%d", i+1),
			part.Name,
			part.FrameConfig.Parent,
			fmt.Sprintf("X:%.0f, Y:%.0f, Z:%.0f", tra.X, tra.Y, tra.Z),
			fmt.Sprintf(
				"Roll:%.2f, Pitch:%.2f, Yaw:%.2f",
				utils.RadToDeg(ori.Roll),
				utils.RadToDeg(ori.Pitch),
				utils.RadToDeg(ori.Yaw),
			),
		})
	}
	return t.Render()
}

// NewMissingParentError returns an error for when a part has named a parent
// whose part is missing from the collection of FrameSystemParts that are undergoing
// topological sorting.
func NewMissingParentError(partName, parentName string) error {
	return fmt.Errorf("part with name %s references non-existent parent %s", partName, parentName)
}

// TopologicallySortParts takes a potentially un-ordered slice of frame system parts and
// sorts them, beginning at the world node.
func TopologicallySortParts(parts Parts) (Parts, error) {
	// set up directory to check existence of parents
	existingParts := make(map[string]bool, len(parts))
	existingParts[referenceframe.World] = true
	for _, part := range parts {
		existingParts[part.Name] = true
	}
	// make map of children
	children := make(map[string]Parts)
	for _, part := range parts {
		parent := part.FrameConfig.Parent
		if !existingParts[parent] {
			return nil, NewMissingParentError(part.Name, parent)
		}
		children[part.FrameConfig.Parent] = append(children[part.FrameConfig.Parent], part)
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
			return children[parent][i].Name < children[parent][j].Name
		}) // sort alphabetically within the topological sort
		for _, part := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, part.Name)
			topoSortedParts = append(topoSortedParts, part)
		}
	}
	return topoSortedParts, nil
}

// renameRemoteParts applies prefixes to frame information if necessary.
func renameRemoteParts(
	remoteParts Parts,
	remoteName string,
	remotePrefix bool,
	connectionName string) Parts {
	for _, p := range remoteParts {
		if p.FrameConfig.Parent == referenceframe.World { // rename World of remote parts
			p.FrameConfig.Parent = connectionName
		}
		if remotePrefix { // rename each non-world part with prefix
			p.Name = remoteName + "." + p.Name
			if p.FrameConfig.Parent != connectionName {
				p.FrameConfig.Parent = remoteName + "." + p.FrameConfig.Parent
			}
		}
	}
	return remoteParts
}

func partMapToPartSlice(partsMap map[string]*config.FrameSystemPart) Parts {
	parts := make([]*config.FrameSystemPart, 0, len(partsMap))
	for _, part := range partsMap {
		parts = append(parts, part)
	}
	return Parts(parts)
}

func partNames(parts Parts) []string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.Name
	}
	return names
}
