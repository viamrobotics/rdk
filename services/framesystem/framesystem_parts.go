package framesystem

import (
	"context"
	"fmt"
	"sort"

	"github.com/edaniels/golog"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/robot"
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

// TopologicallySortParts takes a potentially un-ordered slice of frame system parts and
// sorts them, beginning at the world node.
func TopologicallySortParts(parts Parts) (Parts, error) {
	// make map of children
	children := make(map[string]Parts)
	for _, part := range parts {
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
		return nil, errors.New("there are no frames that connect to a 'world' node. Root node must be named 'world'")
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

// CollectLocalParts collects the physical parts of the robot that may have frame info,
// excluding remote robots and services, etc.
func CollectLocalParts(ctx context.Context, r robot.Robot) (Parts, error) {
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
		if c.Name == referenceframe.World {
			return nil, errors.Errorf("cannot give frame system part the name %s", referenceframe.World)
		}
		if c.Frame.Parent == "" {
			return nil, errors.Errorf("parent field in frame config for part %q is empty", c.Name)
		}
		seen[c.Name] = true
		model, err := extractModelFrameJSON(r, c.ResourceName())
		if err != nil && !errors.Is(err, referenceframe.ErrNoModelInformation) {
			return nil, err
		}
		parts[c.Name] = &config.FrameSystemPart{Name: c.Name, FrameConfig: c.Frame, ModelFrame: model}
	}
	return partMapToPartSlice(parts), nil
}

// CollectAllParts is a helper function to get all parts from the local robot and its  connected remote robots.
func CollectAllParts(
	ctx context.Context,
	r robot.Robot,
	localParts Parts,
	logger golog.Logger) (Parts, error) {
	ctx, span := trace.StartSpan(ctx, "services::framesystem_utils::CollectAllParts")
	defer span.End()
	localRobot, ok := r.(robot.LocalRobot)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("robot.LocalRobot", r)
	}
	parts := Parts{}
	parts = append(parts, localParts...)
	conf, err := localRobot.Config(ctx)
	if err != nil {
		return nil, err
	}
	remoteNames := localRobot.RemoteNames()
	// get frame parts for each of its remotes
	for _, remoteName := range remoteNames {
		remote, ok := localRobot.RemoteByName(remoteName)
		if !ok {
			return nil, errors.Errorf("cannot find remote robot %s", remoteName)
		}
		remoteService, err := FromRobot(remote)
		if err != nil {
			logger.Debugw("remote has frame system error, skipping", "remote", remoteName, "error", err)
			continue
		}
		remoteParts, err := remoteService.Config(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "remote %s", remoteName)
		}
		rConf, err := getRemoteConfig(remoteName, conf)
		if err != nil {
			return nil, errors.Wrapf(err, "remote %s", remoteName)
		}
		if rConf.Frame == nil { // skip over remote if it has no frame info
			logger.Debugf("remote %s has no frame config info, skipping", remoteName)
			continue
		}
		remoteParts = renameRemoteParts(remoteParts, rConf)
		parts = append(parts, remoteParts...)
	}
	return parts, nil
}

// getRemoteConfig gets the parameters for the Remote.
func getRemoteConfig(remoteName string, conf *config.Config) (*config.Remote, error) {
	for _, rConf := range conf.Remotes {
		if rConf.Name == remoteName {
			return &rConf, nil
		}
	}
	return nil, fmt.Errorf("cannot find Remote config with name %q", remoteName)
}

// renameRemoteParts applies prefixes to frame information if necessary.
func renameRemoteParts(remoteParts Parts, remoteConf *config.Remote) Parts {
	connectionName := remoteConf.Name + "_" + referenceframe.World
	for _, p := range remoteParts {
		if p.FrameConfig.Parent == referenceframe.World { // rename World of remote parts
			p.FrameConfig.Parent = connectionName
		}
		if remoteConf.Prefix { // rename each non-world part with prefix
			p.Name = remoteConf.Name + "." + p.Name
			if p.FrameConfig.Parent != connectionName {
				p.FrameConfig.Parent = remoteConf.Name + "." + p.FrameConfig.Parent
			}
		}
	}
	// build the frame system part that connects remote world to base world
	connection := &config.FrameSystemPart{
		Name:        connectionName,
		FrameConfig: remoteConf.Frame,
	}
	remoteParts = append(remoteParts, connection)
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
