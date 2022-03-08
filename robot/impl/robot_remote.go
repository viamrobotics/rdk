package robotimpl

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

var errUnimplemented = errors.New("unimplemented")

// A remoteRobot implements wraps an robot.Robot. It
// assists in the un/prefixing of part names for RemoteRobots that
// are not aware they are integrated elsewhere.
// We intentionally do not promote the underlying robot.Robot
// so that any future changes are forced to consider un/prefixing
// of names.
type remoteRobot struct {
	mu    sync.Mutex
	robot robot.Robot
	conf  config.Remote
	parts *robotParts
}

// newRemoteRobot returns a new remote robot wrapping a given robot.Robot
// and its configuration.
func newRemoteRobot(robot robot.Robot, config config.Remote) *remoteRobot {
	// We pull the parts out here such that we correctly return nil for
	// when parts are accessed. This is because a networked robot client
	// may just return a non-nil wrapper for a part they may not exist.
	remoteParts := partsForRemoteRobot(robot)
	return &remoteRobot{
		robot: robot,
		conf:  config,
		parts: remoteParts,
	}
}

func (rr *remoteRobot) Refresh(ctx context.Context) error {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	refresher, ok := rr.robot.(robot.Refresher)
	if !ok {
		return nil
	}
	if err := refresher.Refresh(ctx); err != nil {
		return err
	}
	rr.parts = partsForRemoteRobot(rr.robot)
	return nil
}

// replace replaces this robot with the given robot. We can do a full
// replacement here since we will always get a full view of the parts,
// not one partially created from a diff.
func (rr *remoteRobot) replace(ctx context.Context, newRobot robot.Robot) {
	rr.mu.Lock()
	defer rr.mu.Unlock()

	actual, ok := newRobot.(*remoteRobot)
	if !ok {
		panic(fmt.Errorf("expected new remote to be %T but got %T", actual, newRobot))
	}

	rr.parts.replaceForRemote(ctx, actual.parts)
}

func (rr *remoteRobot) prefixName(name string) string {
	if rr.conf.Prefix {
		return fmt.Sprintf("%s.%s", rr.conf.Name, name)
	}
	return name
}

func (rr *remoteRobot) unprefixName(name string) string {
	if rr.conf.Prefix {
		return strings.TrimPrefix(name, rr.conf.Name+".")
	}
	return name
}

func (rr *remoteRobot) prefixNames(names []string) []string {
	if !rr.conf.Prefix {
		return names
	}
	newNames := make([]string, 0, len(names))
	for _, name := range names {
		newNames = append(newNames, rr.prefixName(name))
	}
	return newNames
}

func (rr *remoteRobot) prefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.prefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) unprefixResourceName(name resource.Name) resource.Name {
	if !rr.conf.Prefix {
		return name
	}
	newName := rr.unprefixName(name.Name)
	return resource.NewName(
		name.Namespace, name.ResourceType, name.ResourceSubtype, newName,
	)
}

func (rr *remoteRobot) RemoteNames() []string {
	return nil
}

func (rr *remoteRobot) FunctionNames() []string {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	return rr.prefixNames(rr.parts.FunctionNames())
}

func (rr *remoteRobot) ResourceNames() []resource.Name {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	newNames := make([]resource.Name, 0, len(rr.parts.ResourceNames()))
	for _, name := range rr.parts.ResourceNames() {
		name := rr.prefixResourceName(name)
		newNames = append(newNames, name)
	}
	return newNames
}

func (rr *remoteRobot) RemoteByName(name string) (robot.Robot, bool) {
	debug.PrintStack()
	panic(errUnimplemented)
}

func (rr *remoteRobot) ResourceByName(name resource.Name) (interface{}, error) {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	newName := rr.unprefixResourceName(name)
	return rr.parts.ResourceByName(newName)
}

func (rr *remoteRobot) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

func (rr *remoteRobot) Config(ctx context.Context) (*config.Config, error) {
	cfgReal, err := rr.robot.Config(ctx)
	if err != nil {
		return nil, err
	}

	cfg := config.Config{
		Components: make([]config.Component, len(cfgReal.Components)),
	}

	for idx, c := range cfgReal.Components {
		c.Name = rr.prefixName(c.Name)
		if c.Frame != nil {
			c.Frame.Parent = rr.prefixName(c.Frame.Parent)
		}
		cfg.Components[idx] = c
	}

	return &cfg, nil
}

// FrameSystem will return the frame system from the remote robot's server
// remoteRobot may add on its own prefix if specified by the config file.
func (rr *remoteRobot) FrameSystem(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
	if rr.conf.Prefix {
		prefix = rr.prefixName(prefix)
	}
	fs, err := rr.robot.FrameSystem(ctx, name, prefix)
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func (rr *remoteRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *remoteRobot) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rr.robot)
}

// partsForRemoteRobot integrates all parts from a given robot
// except for its remotes. This is for a remote robot to integrate
// which should be unaware of remotes.
// Be sure to update this function if robotParts grows.
func partsForRemoteRobot(robot robot.Robot) *robotParts {
	parts := newRobotParts(robotPartsOptions{}, robot.Logger().Named("parts"))
	for _, name := range robot.FunctionNames() {
		parts.addFunction(name)
	}

	for _, name := range robot.ResourceNames() {
		part, err := robot.ResourceByName(name)
		if err != nil {
			continue
		}
		parts.addResource(name, part)
	}
	return parts
}

// replaceForRemote replaces these parts with the given parts coming from a remote.
func (parts *robotParts) replaceForRemote(ctx context.Context, newParts *robotParts) {
	var oldFunctionNames map[string]struct{}
	var oldResources *resource.Graph

	if len(parts.functions) != 0 {
		oldFunctionNames = make(map[string]struct{}, len(parts.functions))
		for name := range parts.functions {
			oldFunctionNames[name] = struct{}{}
		}
	}

	if len(parts.resources.Nodes) != 0 {
		oldResources = resource.NewGraph()
		for name := range parts.resources.Nodes {
			oldResources.AddNode(name, struct{}{})
		}
	}

	for name, newPart := range newParts.functions {
		_, ok := parts.functions[name]
		delete(oldFunctionNames, name)
		if ok {
			continue
		}
		parts.functions[name] = newPart
	}
	for name, newR := range newParts.resources.Nodes {
		old, ok := parts.resources.Nodes[name]
		if ok {
			oldResources.Remove(name)
			oldPart, oldIsReconfigurable := old.(resource.Reconfigurable)
			newPart, newIsReconfigurable := newR.(resource.Reconfigurable)

			switch {
			case oldIsReconfigurable != newIsReconfigurable:
				// this is an indicator of a serious constructor problem
				// for the resource subtype.
				if oldIsReconfigurable {
					panic(fmt.Errorf(
						"old type %T is reconfigurable whereas new type %T is not",
						old, newR))
				}
				panic(fmt.Errorf(
					"new type %T is reconfigurable whereas old type %T is not",
					newR, old))
			case oldIsReconfigurable && newIsReconfigurable:
				// if we are dealing with a reconfigurable resource
				// use the new resource to reconfigure the old one.
				if err := oldPart.Reconfigure(ctx, newPart); err != nil {
					panic(err)
				}
				continue
			case !oldIsReconfigurable && !newIsReconfigurable:
				// if we are not dealing with a reconfigurable resource
				// we want to close the old resource and replace it with the
				// new.
				if err := utils.TryClose(ctx, old); err != nil {
					panic(err)
				}
			}
		}

		parts.resources.Nodes[name] = newR
	}

	for name := range oldFunctionNames {
		delete(parts.functions, name)
	}
	for name := range oldResources.Nodes {
		parts.resources.Remove(name)
	}
}
