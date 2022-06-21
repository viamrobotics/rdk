package robotimpl

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	rutils "go.viam.com/rdk/utils"
)

// resourceManager manages the actual parts that make up a robot.
type resourceManager struct {
	resources      *resource.Graph
	processManager pexec.ProcessManager
	opts           resourceManagerOptions
}

type resourceUpdateWrapper struct {
	real       interface{}
	isAdded    bool
	isModified bool
	config     config.Component
}

type resourceManagerOptions struct {
	debug              bool
	fromCommand        bool
	allowInsecureCreds bool
	tlsConfig          *tls.Config
}

func (w *resourceUpdateWrapper) Close(ctx context.Context) error {
	if w.real != nil {
		return utils.TryClose(ctx, w.real)
	}
	return nil
}

// newResourceManager returns a properly initialized set of parts.
func newResourceManager(
	opts resourceManagerOptions,
	logger golog.Logger,
) *resourceManager {
	return &resourceManager{
		resources:      resource.NewGraph(),
		processManager: pexec.NewProcessManager(logger),
		opts:           opts,
	}
}

func fromRemoteNameToRemoteNodeName(name string) resource.Name {
	return resource.NewName(resource.ResourceNamespaceRDK, resource.TypeName("remote"), resource.SubtypeName(""), name)
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(ctx context.Context, r robot.Robot, c config.Remote) {
	rName := fromRemoteNameToRemoteNodeName(c.Name)
	manager.addResource(rName, r)
	manager.updateRemoteResourceNames(rName, r)
	fmt.Printf("All childrens are %#v\r\n", manager.resources.GetAllChildrenOf(rName))
}
func (manager *resourceManager) remoteResourceNames(remoteName resource.Name) ([]resource.Name, error) {
	filtered := []resource.Name{}
	if _, ok := manager.resources.Nodes[remoteName]; !ok {
		return nil, errors.Errorf("remote named %q doesn't exist", remoteName.Name)
	}
	children := manager.resources.GetAllChildrenOf(remoteName)
	for _, child := range children {
		if child.IsRemoteResource() {
			filtered = append(filtered, child)
		}
	}
	fmt.Printf("returnning %+v\r\n", children)
	return children, nil
}

func (manager *resourceManager) updateRemoteResourceNames(remoteName resource.Name, r robot.Robot) error {
	visited := make(map[resource.Name]bool)
	newResources := r.ResourceNames()
	oldResources, err := manager.remoteResourceNames(remoteName)
	fmt.Printf("This guy %s send resources %+v had %+v\r\n", remoteName, newResources, oldResources)
	if err != nil {
		return err
	}
	for _, res := range oldResources {
		visited[res] = false
	}
	for _, res := range newResources {
		// skip datamanager since we know it doesn't have a client
		// TODO: remove after we add corresponding datamanager client
		if res == datamanager.Name {
			continue
		}
		rrName := res
		res.PrependRemote(resource.RemoteName(remoteName.Name))
		if _, ok := visited[res]; ok {
			visited[res] = true
			continue
		}
		iface, err := r.ResourceByName(rrName)
		fmt.Printf("AProcessing the following resource %s we have %+v err is %+v\r\n", res, iface, err)
		manager.addResource(res, iface)
		manager.resources.AddChildren(res, remoteName)
	}
	for res, visit := range visited {
		if !visit {
			fmt.Printf("-----=====> Removing %q\r\n", res)
			children := manager.resources.GetAllChildrenOf(res)
			for _, child := range children {
				manager.resources.Remove(child)
				// if err := utils.TryClose(ctx, child); err != nil {
				// 	return err
				// }
			}
			manager.resources.Remove(res)
		}
	}
	return nil
}
func (manager *resourceManager) updateRemotesResourceNames() {
	for name, iface := range manager.resources.Nodes {
		if name.ResourceType == resource.TypeName("remote") {
			if r, ok := iface.(robot.Robot); ok {
				err := manager.updateRemoteResourceNames(name, r)
				if err != nil {
					//TODO logger
					panic(":oops")
				}
			}
		}
	}
}

// addResource adds a resource to the manager.
func (manager *resourceManager) addResource(name resource.Name, r interface{}) {
	manager.resources.AddNode(name, r)
}

// RemoteNames returns the names of all remotes in the manager.
func (manager *resourceManager) RemoteNames() []string {
	names := []string{}
	for k := range manager.resources.Nodes {
		if k.ResourceType == resource.TypeName("remote") {
			names = append(names, k.Name)
		}
	}
	return names
}

// ResourceNames returns the names of all resources in the manager.
func (manager *resourceManager) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for k := range manager.resources.Nodes {
		if k.ResourceType == resource.TypeName("remote") {
			continue
		}
		names = append(names, k)
	}
	return names
}

// remoteNameByResource returns the remote the resource is pulled from, if found.
// False can mean either the resource doesn't exist or is local to the robot.
func (manager *resourceManager) remoteNameByResource(resourceName resource.Name) (string, bool) {
	if !resourceName.IsRemoteResource() {
		return "", false
	}
	remote := strings.Split(string(resourceName.Remote.Remote), ":")
	return remote[0], true
}

// Clone provides a shallow copy of each part.
func (manager *resourceManager) Clone() *resourceManager {
	var clonedManager resourceManager
	if len(manager.resources.Nodes) != 0 {
		clonedManager.resources = manager.resources.Clone()
	}
	if manager.processManager != nil {
		clonedManager.processManager = manager.processManager.Clone()
	}
	clonedManager.opts = manager.opts
	return &clonedManager
}

// Close attempts to close/stop all parts.
func (manager *resourceManager) Close(ctx context.Context) error {
	var allErrs error
	if err := manager.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	order := manager.resources.TopologicalSort()
	for _, x := range order {
		fmt.Printf("Closing %+v\r\n", x)
		if err := utils.TryClose(ctx, manager.resources.Nodes[x]); err != nil {
			fmt.Printf("+++++>>>>---- %+v\r\n", err)
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing resource"))
		}
	}
	return allErrs
}

// processConfig ingests a given config and constructs all constituent parts.
func (manager *resourceManager) processConfig(
	ctx context.Context,
	config *config.Config,
	robot *localRobot,
	logger golog.Logger,
) error {
	if err := manager.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := manager.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := manager.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	if err := manager.newServices(ctx, config.Services, robot); err != nil {
		return err
	}

	return nil
}

// processModifiedConfig ingests a given config and constructs all constituent parts.

// cleanAppImageEnv attempts to revert environent variable changes so
// normal, non-AppImage processes can be executed correctly.
func cleanAppImageEnv() error {
	_, isAppImage := os.LookupEnv("APPIMAGE")
	if isAppImage {
		err := os.Chdir(os.Getenv("APPRUN_CWD"))
		if err != nil {
			return err
		}

		// Reset original values where available
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			key := eVar[0]
			origV, present := os.LookupEnv("APPRUN_ORIGINAL_" + key)
			if present {
				if origV != "" {
					err = os.Setenv(key, origV)
				} else {
					err = os.Unsetenv(key)
				}
				if err != nil {
					return err
				}
			}
		}

		// Remove all explicit appimage vars
		err = multierr.Combine(os.Unsetenv("ARGV0"), os.Unsetenv("ORIGIN"))
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			key := eVar[0]
			if strings.HasPrefix(key, "APPRUN") ||
				strings.HasPrefix(key, "APPDIR") ||
				strings.HasPrefix(key, "APPIMAGE") ||
				strings.HasPrefix(key, "AIX_") {
				err = multierr.Combine(err, os.Unsetenv(key))
			}
		}
		if err != nil {
			return err
		}

		// Remove AppImage paths from path-like env vars
		for _, eVarStr := range os.Environ() {
			eVar := strings.Split(eVarStr, "=")
			var newPaths []string
			const mountPrefix = "/tmp/.mount_"
			key := eVar[0]
			if len(eVar) >= 2 && strings.Contains(eVar[1], mountPrefix) {
				for _, path := range strings.Split(eVar[1], ":") {
					if !strings.HasPrefix(path, mountPrefix) && path != "" {
						newPaths = append(newPaths, path)
					}
				}
				if len(newPaths) > 0 {
					err = os.Setenv(key, strings.Join(newPaths, ":"))
				} else {
					err = os.Unsetenv(key)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// newProcesses constructs all processes defined.
func (manager *resourceManager) newProcesses(ctx context.Context, processes []pexec.ProcessConfig) error {
	// If we're in an AppImage, clean the environment before external execution.
	err := cleanAppImageEnv()
	if err != nil {
		return err
	}

	for _, procConf := range processes {
		if _, err := manager.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
			return err
		}
	}
	return manager.processManager.Start(ctx)
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (manager *resourceManager) newRemotes(ctx context.Context, remotes []config.Remote, logger golog.Logger) error {
	for _, config := range remotes {
		dialOpts := remoteDialOptions(config, manager.opts)
		robotClient, err := dialRobotClient(ctx, config, logger, dialOpts...)
		if err != nil {
			if errors.Is(err, rpc.ErrInsecureWithCredentials) {
				if manager.opts.fromCommand {
					err = errors.New("must use -allow-insecure-creds flag to connect to a non-TLS secured robot")
				} else {
					err = errors.New("must use Config.AllowInsecureCreds to connect to a non-TLS secured robot")
				}
			}
			return errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
		}
		configCopy := config
		manager.addRemote(ctx, robotClient, configCopy)
	}
	return nil
}

// newComponents constructs all components defined.
func (manager *resourceManager) newComponents(ctx context.Context, components []config.Component, robot *localRobot) error {
	for _, c := range components {
		r, err := robot.newResource(ctx, c)
		if err != nil {
			return err
		}
		rName := c.ResourceName()
		manager.addResource(rName, r)
		for _, dep := range c.DependsOn {
			if comp := robot.config.FindComponent(dep); comp != nil {
				if err := manager.resources.AddChildren(rName, comp.ResourceName()); err != nil {
					return err
				}
			} else if name, ok := manager.resources.FindNodeByName(dep); ok {
				if err := manager.resources.AddChildren(rName, *name); err != nil {
					return err
				}
			} else {
				return errors.Errorf("componenent %s depends on non-existent component %s",
					rName.Name, dep)
			}
		}
	}

	return nil
}

// newServices constructs all services defined.
func (manager *resourceManager) newServices(ctx context.Context, services []config.Service, r *localRobot) error {
	for _, c := range services {
		// DataManagerService has to be specifically excluded since it's defined in the config but is a default
		// service that we only want to reconfigure rather than reinstantiate with New().
		if c.ResourceName() == datamanager.Name {
			continue
		}
		svc, err := r.newService(ctx, c)
		if err != nil {
			return err
		}
		manager.addResource(c.ResourceName(), svc)
	}

	return nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (manager *resourceManager) RemoteByName(name string) (robot.Robot, bool) {
	rName := resource.NewName(resource.ResourceNamespaceRDK, resource.TypeName("remote"), resource.SubtypeName(""), name)
	if iface, ok := manager.resources.Nodes[rName]; ok {
		part, ok := iface.(robot.Robot)
		if !ok {
			panic(errors.Errorf("tried to access remote '%q' but its not a robot its %T", name, iface))
		}
		return part, true
	}
	return nil, false
}

func (manager *resourceManager) UpdateConfig(ctx context.Context,
	added *config.Config,
	modified *config.ModifiedConfigDiff,
	logger golog.Logger,
	robot *draftRobot,
) (PartsMergeResult, error) {
	var leftovers PartsMergeResult
	replacedRemotes, err := manager.updateRemotes(ctx, added.Remotes, modified.Remotes, logger)
	leftovers.ReplacedRemotes = replacedRemotes
	if err != nil {
		return leftovers, err
	}
	replacedProcesses, err := manager.updateProcesses(ctx, added.Processes, modified.Processes)
	leftovers.ReplacedProcesses = replacedProcesses
	if err != nil {
		return leftovers, err
	}
	if err := manager.updateServices(ctx, added.Services, modified.Services, robot); err != nil {
		return leftovers, err
	}
	if err := manager.updateComponentsGraph(added.Components, modified.Components, logger, robot); err != nil {
		return leftovers, err
	}
	if err := manager.updateComponents(ctx, logger, robot); err != nil {
		return leftovers, err
	}
	return leftovers, nil
}

func (manager *resourceManager) updateProcesses(ctx context.Context,
	addProcesses []pexec.ProcessConfig,
	modifiedProcesses []pexec.ProcessConfig,
) ([]pexec.ManagedProcess, error) {
	var replacedProcess []pexec.ManagedProcess
	if err := manager.newProcesses(ctx, addProcesses); err != nil {
		return nil, err
	}
	for _, p := range modifiedProcesses {
		old, ok := manager.processManager.ProcessByID(p.ID)
		if !ok {
			return replacedProcess, errors.Errorf("cannot replace non-existing process %q", p.ID)
		}
		if err := manager.newProcesses(ctx, []pexec.ProcessConfig{p}); err != nil {
			return replacedProcess, err
		}
		replacedProcess = append(replacedProcess, old)
	}
	return replacedProcess, nil
}

func (manager *resourceManager) updateRemotes(ctx context.Context,
	addedRemotes []config.Remote,
	modifiedRemotes []config.Remote,
	logger golog.Logger,
) ([]*remoteRobot, error) {
	var replacedRemotes []*remoteRobot
	if err := manager.newRemotes(ctx, addedRemotes, logger); err != nil {
		return nil, err
	}
	for _, r := range modifiedRemotes {
		if err := manager.newRemotes(ctx, []config.Remote{r}, logger); err != nil {
			return replacedRemotes, err
		}
	}
	return replacedRemotes, nil
}

func (manager *resourceManager) reconfigureResource(ctx context.Context, old, newR interface{}) (interface{}, error) {
	oldPart, oldResourceIsReconfigurable := old.(resource.Reconfigurable)
	newPart, newResourceIsReconfigurable := newR.(resource.Reconfigurable)
	switch {
	case oldResourceIsReconfigurable != newResourceIsReconfigurable:
		// this is an indicator of a serious constructor problem
		// for the resource subtype.
		reconfError := errors.Errorf(
			"new type %T is reconfigurable whereas old type %T is not",
			newR, old)
		if oldResourceIsReconfigurable {
			reconfError = errors.Errorf(
				"old type %T is reconfigurable whereas new type %T is not",
				old, newR)
		}
		return nil, reconfError
	case oldResourceIsReconfigurable && newResourceIsReconfigurable:
		// if we are dealing with a reconfigurable resource
		// use the new resource to reconfigure the old one.
		if err := oldPart.Reconfigure(ctx, newPart); err != nil {
			return nil, err
		}
		return old, nil
	case !oldResourceIsReconfigurable && !newResourceIsReconfigurable:
		// if we are not dealing with a reconfigurable resource
		// we want to close the old resource and replace it with the
		// new.
		if err := utils.TryClose(ctx, old); err != nil {
			return nil, err
		}
		return newR, nil
	default:
		return nil, errors.Errorf("unexpected outcome during reconfiguration of type %T and type %T",
			old, newR)
	}
}

func (manager *resourceManager) updateServices(ctx context.Context,
	addedServices []config.Service,
	modifiedServices []config.Service,
	robot *draftRobot,
) error {
	for _, c := range addedServices {
		// DataManagerService has to be specifically excluded since it's defined in the config but is a default
		// service that we only want to reconfigure rather than reinstantiate with New().
		if c.ResourceName() == datamanager.Name {
			continue
		}
		svc, err := robot.newService(ctx, c)
		if err != nil {
			return err
		}
		manager.addResource(c.ResourceName(), svc)
	}
	for _, c := range modifiedServices {
		if c.ResourceName() == datamanager.Name {
			continue
		}
		svc, err := robot.newService(ctx, c)
		if err != nil {
			return err
		}
		old, ok := manager.resources.Nodes[c.ResourceName()]
		if !ok {
			return errors.Errorf("couldn't find %q service while we are trying to modify it", c.ResourceName())
		}
		rr, err := manager.reconfigureResource(ctx, old, svc)
		if err != nil {
			return err
		}
		manager.resources.Nodes[c.ResourceName()] = rr
	}
	return nil
}

func (manager *resourceManager) markChildrenForUpdate(ctx context.Context, rName resource.Name, r *draftRobot) error {
	sg, err := manager.resources.SubGraphFrom(rName)
	if err != nil {
		return err
	}
	sorted := sg.TopologicalSort()
	for _, x := range sorted {
		var originalConfig config.Component
		if _, ok := manager.resources.Nodes[x].(*resourceUpdateWrapper); ok {
			continue
		}
		for _, c := range r.original.config.Components {
			if c.ResourceName() == x {
				originalConfig = c
			}
		}
		if err := utils.TryClose(ctx, manager.resources.Nodes[x]); err != nil {
			return err
		}
		wrapper := &resourceUpdateWrapper{
			real:       nil,
			config:     originalConfig,
			isAdded:    true,
			isModified: false,
		}
		manager.resources.Nodes[x] = wrapper
	}
	return nil
}

func (manager *resourceManager) updateComponent(ctx context.Context,
	rName resource.Name,
	conf config.Component,
	old interface{},
	r *draftRobot,
) error {
	obj, canValidate := old.(config.CompononentUpdate)
	res := config.Rebuild
	if canValidate {
		res = obj.UpdateAction(&conf)
	}
	switch res {
	case config.None:
		return nil
	case config.Reconfigure:
		if err := manager.markChildrenForUpdate(ctx, rName, r); err != nil {
			return err
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return err
		}
		rr, err := manager.reconfigureResource(ctx, old, nr)
		if err != nil {
			return err
		}
		manager.resources.Nodes[rName] = rr
	case config.Rebuild:
		if err := manager.markChildrenForUpdate(ctx, rName, r); err != nil {
			return err
		}
		if err := utils.TryClose(ctx, old); err != nil {
			return err
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return err
		}
		manager.resources.Nodes[rName] = nr
	}
	return nil
}

func (manager *resourceManager) updateComponents(ctx context.Context,
	logger golog.Logger,
	robot *draftRobot,
) error {
	sorted := manager.resources.ReverseTopologicalSort()
	for _, c := range sorted {
		wrapper, ok := manager.resources.Nodes[c].(*resourceUpdateWrapper)
		if !ok {
			continue
		}
		if wrapper.isAdded {
			logger.Infof("building the component %q with config %+v", c.Name, wrapper.config)
			r, err := robot.newResource(ctx, wrapper.config)
			if err != nil {
				return err
			}
			manager.resources.Nodes[c] = r
		} else if wrapper.isModified {
			err := manager.updateComponent(ctx, c, wrapper.config, wrapper.real, robot)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (manager *resourceManager) updateComponentsGraph(addedComponents []config.Component,
	modifiedComponents []config.Component,
	logger golog.Logger,
	robot *draftRobot,
) error {
	// Assumptions :
	// added & modified slices are ordered
	for _, add := range addedComponents {
		rName := add.ResourceName()
		wrapper := &resourceUpdateWrapper{
			real:       nil,
			isAdded:    true,
			isModified: false,
			config:     add,
		}
		if manager.resources.Nodes[rName] != nil {
			logger.Debugf("%q is already exists", rName)
			return errors.Errorf("cannot add component %q it already exists", rName)
		}
		manager.addResource(rName, wrapper)
		for _, dep := range add.DependsOn {
			if comp := robot.original.config.FindComponent(dep); comp != nil {
				if err := manager.resources.AddChildren(rName, comp.ResourceName()); err != nil {
					return err
				}
			} else if name, ok := manager.resources.FindNodeByName(dep); ok {
				if err := manager.resources.AddChildren(rName, *name); err != nil {
					return err
				}
			} else {
				return errors.Errorf("componenent %s depends on non-existent component %s",
					rName.Name, dep)
			}
		}
	}
	for _, modif := range modifiedComponents {
		rName := modif.ResourceName()
		if _, ok := manager.resources.Nodes[rName]; !ok {
			return errors.Errorf("cannot modify non-existent component %q", rName)
		}
		wrapper := &resourceUpdateWrapper{
			real:       manager.resources.Nodes[rName],
			isAdded:    false,
			isModified: true,
			config:     modif,
		}
		manager.resources.Nodes[rName] = wrapper
		parents := manager.resources.GetAllParentsOf(rName)
		mapParents := make(map[resource.Name]bool)
		for _, pdep := range parents {
			mapParents[pdep] = false
		}
		// parse the dependency tree and optionally add/remove components
		for _, dep := range modif.DependsOn {
			var comp resource.Name
			if r := robot.original.config.FindComponent(dep); r != nil {
				comp = r.ResourceName()
			} else if r, ok := manager.resources.FindNodeByName(dep); ok {
				comp = *r
			} else {
				return errors.Errorf("component %q depends on non-existent component %q",
					rName.Name, dep)
			}
			if _, ok := mapParents[comp]; ok {
				mapParents[comp] = true
				continue
			}
			if err := manager.resources.AddChildren(rName, comp); err != nil {
				return err
			}
		}
		for k, v := range mapParents {
			if v {
				continue
			}
			if err := manager.resources.RemoveChildren(rName, k); err != nil {
				return err
			}
		}
	}
	return nil
}

// ResourceByName returns the given resource by fully qualified name, if it exists;
// returns an error otherwise.
func (manager *resourceManager) ResourceByName(name resource.Name) (interface{}, error) {
	robotPart, ok := manager.resources.Nodes[name]
	if ok {
		return robotPart, nil
	}
	return nil, rutils.NewResourceNotFoundError(name)
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
	ReplacedRemotes   []*remoteRobot
}

// FilterFromConfig given a config this function will remove elements from the manager and return removed resources in a new manager.
func (manager *resourceManager) FilterFromConfig(ctx context.Context, conf *config.Config, logger golog.Logger) (*resourceManager, error) {
	filtered := newResourceManager(manager.opts, logger)

	for _, conf := range conf.Processes {
		proc, ok := manager.processManager.ProcessByID(conf.ID)
		if !ok {
			continue
		}
		if _, err := filtered.processManager.AddProcess(ctx, proc, false); err != nil {
			return nil, err
		}
		manager.processManager.RemoveProcessByID(conf.ID)
	}
	for _, conf := range conf.Remotes {
		remoteName := fromRemoteNameToRemoteNodeName(conf.Name)
		iface, ok := manager.resources.Nodes[remoteName]
		if !ok {
			continue
		}
		part, ok := iface.(robot.Robot)
		if !ok {
			return nil, errors.Errorf("remote named %q exists but its a %T and not a robotClient", remoteName, iface)
		}
		filtered.resources.AddNode(remoteName, part)
		for _, child := range manager.resources.GetAllChildrenOf(remoteName) {
			if _, ok := filtered.resources.Nodes[child]; !ok {
				filtered.resources.AddNode(child, manager.resources.Nodes[child])
			}
			if err := filtered.resources.AddChildren(child, remoteName); err != nil {
				return nil, err
			}
		}
		//TODO also remove children
		manager.resources.Remove(remoteName)
	}
	for _, compConf := range conf.Components {
		rName := compConf.ResourceName()
		_, err := manager.ResourceByName(rName)
		if err != nil {
			continue
		}
		filtered.resources.AddNode(rName, manager.resources.Nodes[rName])
		for _, child := range manager.resources.GetAllChildrenOf(rName) {
			if _, ok := filtered.resources.Nodes[child]; !ok {
				filtered.resources.AddNode(child, manager.resources.Nodes[child])
			}
			if err := filtered.resources.AddChildren(child, rName); err != nil {
				return nil, err
			}
		}
		manager.resources.Remove(rName)
	}
	for _, conf := range conf.Services {
		rName := conf.ResourceName()
		_, err := manager.ResourceByName(rName)
		if err != nil {
			continue
		}
		for _, child := range manager.resources.GetAllChildrenOf(rName) {
			if err := filtered.resources.AddChildren(child, rName); err != nil {
				return nil, err
			}
			filtered.resources.Nodes[child] = manager.resources.Nodes[child]
		}
		manager.resources.Remove(rName)
	}
	return filtered, nil
}
