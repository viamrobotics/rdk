// Package framesystemparts provides functionality around a list of framesystem parts
package framesystemparts

import (
	"fmt"
	"sort"

	"github.com/golang/geo/r3"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Parts is a slice of *config.FrameSystemPart.
type Parts []*referenceframe.FrameSystemPart

// String prints out a table of each frame in the system, with columns of name, parent, translation and orientation.
func (fsp Parts) String() string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"#", "Name", "Parent", "Translation", "Orientation", "Geometry"})
	t.AppendRow([]interface{}{"0", referenceframe.World, "", "", "", ""})
	for i, part := range fsp {
		tra := part.FrameConfig.Translation
		ori := &spatialmath.EulerAngles{}
		pose, err := part.FrameConfig.Pose()
		if err == nil {
			ori = pose.Orientation().EulerAngles()
		}
		geomString := ""
		if part.FrameConfig.Geometry != nil {
			gCfg := part.FrameConfig.Geometry
			creator, err := gCfg.ParseConfig()
			if err == nil {
				switch gCfg.Type {
				case spatialmath.BoxType:
					geomString = fmt.Sprintf("Type: Box Dim: X:%.0f, Y:%.0f, Z:%.0f", gCfg.X, gCfg.Y, gCfg.Z)
				case spatialmath.SphereType:
					geomString = fmt.Sprintf("Type: Sphere Radius: %.0f", gCfg.R)
				case spatialmath.PointType:
					geomString = fmt.Sprintf(
						"Type: Point Loc X:%.0f, Y:%.0f, Z:%.0f",
						gCfg.TranslationOffset.X,
						gCfg.TranslationOffset.Y,
						gCfg.TranslationOffset.Z,
					)
				case spatialmath.UnknownType:
					// no type specified, iterate through supported types and try to infer intent
					if _, err := spatialmath.NewBoxCreator(
						r3.Vector{X: gCfg.X, Y: gCfg.Y, Z: gCfg.Z},
						creator.Offset(),
						gCfg.Label,
					); err == nil {
						geomString = fmt.Sprintf("Type: Box Dim: X:%.0f, Y:%.0f, Z:%.0f", gCfg.X, gCfg.Y, gCfg.Z)
					}
					if _, err := spatialmath.NewSphereCreator(gCfg.R, creator.Offset(), gCfg.Label); err == nil {
						geomString = fmt.Sprintf("Type: Sphere Radius: %.0f", gCfg.R)
					}
					// never try to infer point geometry if nothing is specified
				}
			}
		}
		t.AppendRow([]interface{}{
			fmt.Sprintf("%d", i+1),
			part.FrameConfig.ID,
			part.FrameConfig.Parent,
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

// NewMissingParentError returns an error for when a part has named a parent
// whose part is missing from the collection of FrameSystemParts that are undergoing
// topological sorting.
func NewMissingParentError(partName, parentName string) error {
	return fmt.Errorf("part with name %s references non-existent parent %s", partName, parentName)
}

// TopologicallySort takes a potentially un-ordered slice of frame system parts and
// sorts them, beginning at the world node.
func TopologicallySort(parts Parts) (Parts, error) {
	// set up directory to check existence of parents
	existingParts := make(map[string]bool, len(parts))
	existingParts[referenceframe.World] = true
	for _, part := range parts {
		existingParts[part.FrameConfig.ID] = true
	}
	// make map of children
	children := make(map[string]Parts)
	// ~ fmt.Println("parts", parts)
	for _, part := range parts {
		// ~ fmt.Println("part", part)
		parent := part.FrameConfig.Parent
		if !existingParts[parent] {
			return nil, NewMissingParentError(part.FrameConfig.ID, parent)
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
			return children[parent][i].FrameConfig.ID < children[parent][j].FrameConfig.ID
		}) // sort alphabetically within the topological sort
		for _, part := range children[parent] { // add all the children to the frame system, and to the stack as new parents
			stack = append(stack, part.FrameConfig.ID)
			topoSortedParts = append(topoSortedParts, part)
		}
	}
	return topoSortedParts, nil
}

// RenameRemoteParts applies prefixes to frame information if necessary.
func RenameRemoteParts(
	remoteParts Parts,
	remoteName string,
	connectionName string,
) Parts {
	for _, p := range remoteParts {
		if p.FrameConfig.Parent == referenceframe.World { // rename World of remote parts
			p.FrameConfig.Parent = connectionName
		}
		// rename each non-world part with prefix
		p.FrameConfig.ID = remoteName + ":" + p.FrameConfig.ID
		if p.FrameConfig.Parent != connectionName {
			p.FrameConfig.Parent = remoteName + ":" + p.FrameConfig.Parent
		}
	}
	return remoteParts
}

// PartMapToPartSlice returns a Parts constructed of the FrameSystemParts values of a string map.
func PartMapToPartSlice(partsMap map[string]*referenceframe.FrameSystemPart) Parts {
	parts := make([]*referenceframe.FrameSystemPart, 0, len(partsMap))
	for _, part := range partsMap {
		parts = append(parts, part)
	}
	return Parts(parts)
}

// Names returns the names of input parts.
func Names(parts Parts) []string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.FrameConfig.ID
	}
	return names
}
