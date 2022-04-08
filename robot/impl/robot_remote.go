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
	mu      sync.Mutex
	robot   robot.Robot
	conf    config.Remote
	manager *resourceManager
}

// newRemoteRobot returns a new remote robot wrapping a given robot.Robot
// and its configuration.
func newRemoteRobot(robot robot.Robot, config config.Remote) *remoteRobot {
	// We pull the manager out here such that we correctly return nil for
	// when parts are accessed. This is because a networked robot client
	// may just return a non-nil wrapper for a part they may not exist.
	remoteManager := managerForRemoteRobot(robot)
	return &remoteRobot{
		robot:   robot,
		conf:    config,
		manager: remoteManager,
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
	rr.manager = managerForRemoteRobot(rr.robot)
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

	rr.manager.replaceForRemote(ctx, actual.manager)
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
	return rr.prefixNames(rr.manager.FunctionNames())
}

func (rr *remoteRobot) ResourceNames() []resource.Name {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	newNames := make([]resource.Name, 0, len(rr.manager.ResourceNames()))
	for _, name := range rr.manager.ResourceNames() {
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
	return rr.manager.ResourceByName(newName)
}

func (rr *remoteRobot) ProcessManager() pexec.ProcessManager {
	return pexec.NoopProcessManager
}

func (rr *remoteRobot) Logger() golog.Logger {
	return rr.robot.Logger()
}

func (rr *remoteRobot) Close(ctx context.Context) error {
	return utils.TryClose(ctx, rr.robot)
}

// managerForRemoteRobot integrates all parts from a given robot
// except for its remotes. This is for a remote robot to integrate
// which should be unaware of remotes.
// Be sure to update this function if resourceManager grows.
func managerForRemoteRobot(robot robot.Robot) *resourceManager {
	manager := newResourceManager(resourceManagerOptions{}, robot.Logger().Named("manager"))
	for _, name := range robot.FunctionNames() {
		manager.addFunction(name)
	}

	for _, name := range robot.ResourceNames() {
		part, err := robot.ResourceByName(name)
		if err != nil {
			robot.Logger().Debugw("error getting resource", "resource", name, "error", err)
			continue
		}
		manager.addResource(name, part)
	}
	return manager
}

// replaceForRemote replaces these parts with the given parts coming from a remote.
func (manager *resourceManager) replaceForRemote(ctx context.Context, newManager *resourceManager) {
	var oldFunctionNames map[string]struct{}
	var oldResources *resource.Graph

	if len(manager.functions) != 0 {
		oldFunctionNames = make(map[string]struct{}, len(manager.functions))
		for name := range manager.functions {
			oldFunctionNames[name] = struct{}{}
		}
	}

	if len(manager.resources.Nodes) != 0 {
		oldResources = resource.NewGraph()
		for name := range manager.resources.Nodes {
			oldResources.AddNode(name, struct{}{})
		}
	}

	for name, newPart := range newManager.functions {
		_, ok := manager.functions[name]
		delete(oldFunctionNames, name)
		if ok {
			continue
		}
		manager.functions[name] = newPart
	}
	for name, newR := range newManager.resources.Nodes {
		old, ok := manager.resources.Nodes[name]
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

		manager.resources.Nodes[name] = newR
	}

	for name := range oldFunctionNames {
		delete(manager.functions, name)
	}
	for name := range oldResources.Nodes {
		manager.resources.Remove(name)
	}
}
