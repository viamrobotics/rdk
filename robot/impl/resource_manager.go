package robotimpl

import (
	"context"
	"crypto/tls"
	"os"
	"strings"

	"github.com/edaniels/golog"
	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	rutils "go.viam.com/rdk/utils"
)

const remoteTypeName = resource.TypeName("remote")

var remoteSubtype = resource.NewSubtype(resource.ResourceNamespaceRDK,
	remoteTypeName,
	resource.SubtypeName(""))

// resourceManager manages the actual parts that make up a robot.
type resourceManager struct {
	resources      *resource.Graph
	processManager pexec.ProcessManager
	opts           resourceManagerOptions
	logger         golog.Logger
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
		logger:         logger,
	}
}

func fromRemoteNameToRemoteNodeName(name string) resource.Name {
	return resource.NameFromSubtype(remoteSubtype, name)
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(ctx context.Context, r robot.Robot, c config.Remote) {
	rName := fromRemoteNameToRemoteNodeName(c.Name)
	manager.addResource(rName, r)
	manager.updateRemoteResourceNames(ctx, rName, r)
}

func (manager *resourceManager) remoteResourceNames(remoteName resource.Name) []resource.Name {
	var filtered []resource.Name
	if _, ok := manager.resources.Nodes[remoteName]; !ok {
		manager.logger.Errorw("trying to get remote resources of a non existing remote", "remote", remoteName)
	}
	children := manager.resources.GetAllChildrenOf(remoteName)
	for _, child := range children {
		if child.ContainsRemoteNames() {
			filtered = append(filtered, child)
		}
	}
	return filtered
}

func (manager *resourceManager) updateRemoteResourceNames(ctx context.Context, remoteName resource.Name, r robot.Robot) {
	visited := map[resource.Name]bool{}
	newResources := r.ResourceNames()
	oldResources := manager.remoteResourceNames(remoteName)
	for _, res := range oldResources {
		visited[res] = false
	}
	for _, res := range newResources {
		rrName := res
		res = res.PrependRemote(resource.RemoteName(remoteName.Name))
		if _, ok := visited[res]; ok {
			visited[res] = true
			continue
		}
		iface, err := r.ResourceByName(rrName)
		if err != nil {
			manager.logger.Errorw("couldn't obtain remote resource interface",
				"name", rrName,
				"reason", err)
			continue
		}
		manager.addResource(res, iface)
		err = manager.resources.AddChildren(res, remoteName)
		if err != nil {
			manager.logger.Errorf("error while trying add %q as a dependency of remote %q", res, remoteName)
		}
	}
	for res, visit := range visited {
		if !visit {
			// TODO(npmenard) Add test case when implementing RSDK-435
			sg, err := manager.resources.SubGraphFrom(res)
			if err != nil {
				manager.logger.Errorw("failed to generate a subgraph from the resource", "resource", res, "reason", err)
				continue
			}
			sorted := sg.TopologicalSort()
			for _, child := range sorted {
				if err := utils.TryClose(ctx, child); err != nil {
					manager.logger.Errorw("error while trying to remove a node depending on a delete remote resource",
						"remote", res,
						"node", child,
						"error", err)
				}
				manager.resources.Remove(child)
			}
		}
	}
}

func (manager *resourceManager) updateRemotesResourceNames(ctx context.Context) {
	for name, iface := range manager.resources.Nodes {
		if name.ResourceType == remoteTypeName {
			if r, ok := iface.(robot.Robot); ok {
				manager.updateRemoteResourceNames(ctx, name, r)
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
		if k.ResourceType == remoteTypeName {
			names = append(names, k.Name)
		}
	}
	return names
}

// ResourceNames returns the names of all resources in the manager.
func (manager *resourceManager) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for k := range manager.resources.Nodes {
		if k.ResourceType == remoteTypeName {
			continue
		}
		names = append(names, k)
	}
	return names
}

// ResourceRPCSubtypes returns the types of all resource RPC subtypes in use by the manager.
func (manager *resourceManager) ResourceRPCSubtypes() []resource.RPCSubtype {
	resourceSubtypes := registry.RegisteredResourceSubtypes()

	types := map[resource.Subtype]*desc.ServiceDescriptor{}
	for k := range manager.resources.Nodes {
		if k.ResourceType == remoteTypeName {
			rr, ok := manager.resources.Nodes[k].(robot.Robot)
			if !ok {
				manager.logger.Errorw("remote robot doesn't implement the robot interface",
					"remote", k,
					"type", manager.resources.Nodes[k])
				continue
			}
			manager.mergeResourceRPCSubtypesWithRemote(rr, types)
			continue
		}
		if k.ContainsRemoteNames() {
			continue
		}
		if types[k.Subtype] != nil {
			continue
		}

		st, ok := resourceSubtypes[k.Subtype]
		if !ok {
			continue
		}

		types[k.Subtype] = st.ReflectRPCServiceDesc
	}
	typesList := make([]resource.RPCSubtype, 0, len(types))
	for k, v := range types {
		typesList = append(typesList, resource.RPCSubtype{
			Subtype: k,
			Desc:    v,
		})
	}
	return typesList
}

// mergeResourceRPCSubtypesWithRemotes merges types from the manager itself as well as its
// remotes.
func (manager *resourceManager) mergeResourceRPCSubtypesWithRemote(r robot.Robot, types map[resource.Subtype]*desc.ServiceDescriptor) {
	remoteTypes := r.ResourceRPCSubtypes()
	for _, remoteType := range remoteTypes {
		if svcName, ok := types[remoteType.Subtype]; ok {
			if svcName.GetFullyQualifiedName() != remoteType.Desc.GetFullyQualifiedName() {
				manager.logger.Errorw(
					"remote proto service name clashes with another of same subtype",
					"existing", svcName.GetFullyQualifiedName(),
					"remote", remoteType.Desc.GetFullyQualifiedName())
			}
			continue
		}
		types[remoteType.Subtype] = remoteType.Desc
	}
}

// remoteNameByResource returns the remote the resource is pulled from, if found.
// False can mean either the resource doesn't exist or is local to the robot.
func (manager *resourceManager) remoteNameByResource(resourceName resource.Name) (string, bool) {
	if !resourceName.ContainsRemoteNames() {
		return "", false
	}
	remote := strings.Split(string(resourceName.Remote), ":")
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
		if err := utils.TryClose(ctx, manager.resources.Nodes[x]); err != nil {
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
) {
	manager.newProcesses(ctx, config.Processes)

	manager.newRemotes(ctx, config.Remotes, robot)

	manager.newComponents(ctx, config.Components, robot)

	manager.newServices(ctx, config.Services, robot)
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
func (manager *resourceManager) newProcesses(ctx context.Context, processes []pexec.ProcessConfig) {
	// If we're in an AppImage, clean the environment before external execution.
	err := cleanAppImageEnv()
	if err != nil {
		manager.logger.Errorw("failed to properly clean AppImage", "error", err)
	}

	for _, procConf := range processes {
		if err := manager.newProcess(ctx, procConf); err != nil {
			manager.logger.Errorw("failed to create new process", "error", err)
		}
	}

	err = manager.processManager.Start(ctx)
	if err != nil {
		manager.logger.Errorw("there are process(es) that failed to start", "error", err)
	}
}

func (manager *resourceManager) newProcess(ctx context.Context, procConf pexec.ProcessConfig) error {
	if _, err := manager.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
		return err
	}
	return nil
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (manager *resourceManager) newRemotes(ctx context.Context,
	remotes []config.Remote,
	robot *localRobot,
) {
	for _, config := range remotes {
		err := manager.newRemote(ctx, config, robot)
		if err != nil {
			manager.logger.Errorw("couldn't connect to remote", "name", config.Name, "error", err)
		}
	}
}

// newRemote construct a single remote and integerates its parts in.
func (manager *resourceManager) newRemote(ctx context.Context,
	config config.Remote,
	robot *localRobot,
) error {
	dialOpts := remoteDialOptions(config, manager.opts)
	robotClient, err := dialRobotClient(ctx, config, manager.logger, dialOpts...)
	if err != nil {
		if errors.Is(err, rpc.ErrInsecureWithCredentials) {
			if manager.opts.fromCommand {
				err = errors.New("must use -allow-insecure-creds flag to connect to a non-TLS secured robot")
			} else {
				err = errors.New("must use Config.AllowInsecureCreds to connect to a non-TLS secured robot")
			}
		}
		return err
	}

	configCopy := config
	manager.addRemote(ctx, robotClient, configCopy)
	robotClient.SetParentNotifier(func() {
		rName := fromRemoteNameToRemoteNodeName(configCopy.Name)
		if robot.closeContext.Err() != nil {
			return
		}
		select {
		case <-robot.closeContext.Done():
			return
		case robot.remotesChanged <- rName:
		}
	})
	return nil
}

// newComponents constructs all components defined.
func (manager *resourceManager) newComponents(ctx context.Context, components []config.Component, robot *localRobot) {
	for _, c := range components {
		err := manager.newComponent(ctx, c, robot)
		if err != nil {
			manager.logger.Errorw("failed to add new component", "component", c.ResourceName(), "error", err)
		}
	}
}

func (manager *resourceManager) newComponent(ctx context.Context, c config.Component, robot *localRobot) error {
	r, err := robot.newResource(ctx, c)
	if err != nil {
		return err
	}
	rName := c.ResourceName()
	manager.addResource(rName, r)
	for _, dep := range c.Dependencies() {
		err := manager.newComponentDependency(dep, robot, rName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (manager *resourceManager) newComponentDependency(dep string, robot *localRobot, rName resource.Name) error {
	if comp := robot.config.FindComponent(dep); comp != nil {
		if err := manager.resources.AddChildren(rName, comp.ResourceName()); err != nil {
			return err
		}
	} else if name, ok := manager.resources.FindNodeByName(dep); ok {
		if err := manager.resources.AddChildren(rName, *name); err != nil {
			return err
		}
	} else {
		return errors.Errorf("component %s depends on non-existent component", rName.Name)
	}
	return nil
}

// newServices constructs all services defined.
func (manager *resourceManager) newServices(ctx context.Context, services []config.Service, r *localRobot) {
	for _, c := range services {
		err := manager.newService(ctx, c, r)
		if err != nil {
			manager.logger.Errorw("failed to add new service", "service", c.ResourceName(), "error", err)
		}
	}
}

func (manager *resourceManager) newService(ctx context.Context, cs config.Service, r *localRobot) error {
	// DataManagerService has to be specifically excluded since it's defined in the config but is a default
	// service that we only want to reconfigure rather than reinstantiate with New().
	if cs.ResourceName() == datamanager.Name {
		return nil
	}
	svc, err := r.newService(ctx, cs)
	if err != nil {
		return err
	}
	manager.addResource(cs.ResourceName(), svc)
	return nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (manager *resourceManager) RemoteByName(name string) (robot.Robot, bool) {
	rName := resource.NameFromSubtype(remoteSubtype, name)
	if iface, ok := manager.resources.Nodes[rName]; ok {
		part, ok := iface.(robot.Robot)
		if !ok {
			manager.logger.Errorf("tried to access remote '%q' but its not a robot interface its %T", name, iface)
			return nil, false
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
	manager.updateRemotes(ctx, added.Remotes, modified.Remotes, robot.original)

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
	manager.newProcesses(ctx, addProcesses)
	for _, p := range modifiedProcesses {
		old, ok := manager.processManager.ProcessByID(p.ID)
		if !ok {
			return replacedProcess, errors.Errorf("cannot replace non-existing process %q", p.ID)
		}
		manager.newProcesses(ctx, []pexec.ProcessConfig{p})
		replacedProcess = append(replacedProcess, old)
	}
	return replacedProcess, nil
}

func (manager *resourceManager) updateRemotes(ctx context.Context,
	addedRemotes []config.Remote,
	modifiedRemotes []config.Remote,
	robot *localRobot,
) {
	manager.newRemotes(ctx, addedRemotes, robot)
	for _, r := range modifiedRemotes {
		manager.newRemotes(ctx, []config.Remote{r}, robot)
	}
}

func (manager *resourceManager) reconfigureResource(ctx context.Context, old, newR interface{}) (interface{}, error) {
	if old == nil {
		// if the oldPart was never created, replace directly with the new resource
		return newR, nil
	}

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
			manager.resources.Nodes[c.ResourceName()] = svc
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
				return errors.Errorf("component %s depends on non-existent component %s",
					rName.Name, dep)
			}
		}
	}
	for _, modif := range modifiedComponents {
		rName := modif.ResourceName()
		_, ok := manager.resources.Nodes[rName]
		if !ok {
			manager.addResource(rName, &resourceUpdateWrapper{
				real:       nil,
				isAdded:    true,
				isModified: false,
				config:     modif,
			})
		} else {
			wrapper := &resourceUpdateWrapper{
				real:       manager.resources.Nodes[rName],
				isAdded:    false,
				isModified: true,
				config:     modif,
			}
			manager.resources.Nodes[rName] = wrapper
		}

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
	// if we haven't found a resource of this name then we are going to look into remote resources to find it.
	if !ok && !name.ContainsRemoteNames() {
		keys := manager.resources.FindNodesByShortNameAndSubtype(name)
		if len(keys) > 1 {
			return nil, rutils.NewRemoteResourceClashError(name.Name)
		}
		if len(keys) == 1 {
			robotPart := manager.resources.Nodes[keys[0]]
			return robotPart, nil
		}
	}
	return nil, rutils.NewResourceNotFoundError(name)
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
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
		// TODO also remove children
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

func remoteDialOptions(config config.Remote, opts resourceManagerOptions) []rpc.DialOption {
	var dialOpts []rpc.DialOption
	if opts.debug {
		dialOpts = append(dialOpts, rpc.WithDialDebug())
	}
	if config.Insecure {
		dialOpts = append(dialOpts, rpc.WithInsecure())
	}
	if opts.allowInsecureCreds {
		dialOpts = append(dialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
	}
	if opts.tlsConfig != nil {
		dialOpts = append(dialOpts, rpc.WithTLSConfig(opts.tlsConfig))
	}
	if config.Auth.Credentials != nil {
		if config.Auth.Entity == "" {
			dialOpts = append(dialOpts, rpc.WithCredentials(*config.Auth.Credentials))
		} else {
			dialOpts = append(dialOpts, rpc.WithEntityCredentials(config.Auth.Entity, *config.Auth.Credentials))
		}
	} else {
		// explicitly unset credentials so they are not fed to remotes unintentionally.
		dialOpts = append(dialOpts, rpc.WithEntityCredentials("", rpc.Credentials{}))
	}

	if config.Auth.ExternalAuthAddress != "" {
		dialOpts = append(dialOpts, rpc.WithExternalAuth(
			config.Auth.ExternalAuthAddress,
			config.Auth.ExternalAuthToEntity,
		))
	}

	if config.Auth.ExternalAuthInsecure {
		dialOpts = append(dialOpts, rpc.WithExternalAuthInsecure())
	}

	if config.Auth.SignalingServerAddress != "" {
		wrtcOpts := rpc.DialWebRTCOptions{
			Config:                 &rpc.DefaultWebRTCConfiguration,
			SignalingServerAddress: config.Auth.SignalingServerAddress,
			SignalingAuthEntity:    config.Auth.SignalingAuthEntity,
		}
		if config.Auth.SignalingCreds != nil {
			wrtcOpts.SignalingCreds = *config.Auth.SignalingCreds
		}
		dialOpts = append(dialOpts, rpc.WithWebRTCOptions(wrtcOpts))

		if config.Auth.Managed {
			// managed robots use TLS authN/Z
			dialOpts = append(dialOpts, rpc.WithDialMulticastDNSOptions(rpc.DialMulticastDNSOptions{
				RemoveAuthCredentials: true,
			}))
		}
	}
	return dialOpts
}
