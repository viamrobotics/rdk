package robotimpl

import (
	"context"
	"crypto/tls"
	"os"
	"strings"

	"github.com/alessio/shellescape"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/grpc/client"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/datamanager"
	rutils "go.viam.com/rdk/utils"
)

// resourceManager manages the actual parts that make up a robot.
type resourceManager struct {
	remotes        map[string]*remoteRobot
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

// newResourceManager returns a properly initialized set of parts.
func newResourceManager(
	opts resourceManagerOptions,
	logger golog.Logger,
) *resourceManager {
	return &resourceManager{
		remotes:        map[string]*remoteRobot{},
		resources:      resource.NewGraph(),
		processManager: pexec.NewProcessManager(logger),
		opts:           opts,
	}
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(r *remoteRobot, c config.Remote) {
	manager.remotes[c.Name] = r
}

// addResource adds a resource to the manager.
func (manager *resourceManager) addResource(name resource.Name, r interface{}) {
	manager.resources.AddNode(name, r)
}

// RemoteNames returns the names of all remotes in the manager.
func (manager *resourceManager) RemoteNames() []string {
	names := []string{}
	for k := range manager.remotes {
		names = append(names, k)
	}
	return names
}

// mergeResourceNamesWithRemotes merges names from the manager itself as well as its
// remotes.
func (manager *resourceManager) mergeResourceNamesWithRemotes(names []resource.Name) []resource.Name {
	// use this to filter out seen names and preserve order
	seen := make(map[resource.Name]struct{}, len(manager.resources.Nodes))
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range manager.remotes {
		remoteNames := r.ResourceNames()
		for _, name := range remoteNames {
			if _, ok := seen[name]; ok {
				continue
			}
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	return names
}

// ResourceNames returns the names of all resources in the manager.
func (manager *resourceManager) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for k := range manager.resources.Nodes {
		names = append(names, k)
	}
	return manager.mergeResourceNamesWithRemotes(names)
}

// Clone provides a shallow copy of each part.
func (manager *resourceManager) Clone() *resourceManager {
	var clonedManager resourceManager
	if len(manager.remotes) != 0 {
		clonedManager.remotes = make(map[string]*remoteRobot, len(manager.remotes))
		for k, v := range manager.remotes {
			clonedManager.remotes[k] = v
		}
	}
	if len(manager.resources.Nodes) != 0 {
		clonedManager.resources = manager.resources.Clone()
	}
	if manager.processManager != nil {
		clonedManager.processManager = manager.processManager.Clone()
	}
	return &clonedManager
}

// Close attempts to close/stop all parts.
func (manager *resourceManager) Close(ctx context.Context) error {
	var allErrs error
	if err := manager.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	for _, x := range manager.remotes {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing remote"))
		}
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

// newProcesses constructs all processes defined.
func (manager *resourceManager) newProcesses(ctx context.Context, processes []pexec.ProcessConfig) error {
	for _, procConf := range processes {
		// In an AppImage execve() is meant to be hooked to swap out the AppImage's libraries and the system ones.
		// Go doesn't use libc's execve() though, so the hooks fail and trying to exec binaries outside the AppImage can fail.
		// We work around this by execing through a bash shell (included in the AppImage) which then gets hooked properly.
		_, isAppImage := os.LookupEnv("APPIMAGE")
		if isAppImage {
			procConf.Args = []string{"-c", shellescape.QuoteCommand(append([]string{procConf.Name}, procConf.Args...))}
			procConf.Name = "bash"
		}

		if _, err := manager.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
			return err
		}
	}
	return manager.processManager.Start(ctx)
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (manager *resourceManager) newRemotes(ctx context.Context, remotes []config.Remote, logger golog.Logger) error {
	for _, config := range remotes {
		var dialOpts []rpc.DialOption
		if manager.opts.debug {
			dialOpts = append(dialOpts, rpc.WithDialDebug())
		}
		if config.Insecure {
			dialOpts = append(dialOpts, rpc.WithInsecure())
		}
		if manager.opts.allowInsecureCreds {
			dialOpts = append(dialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
		}
		if manager.opts.tlsConfig != nil {
			dialOpts = append(dialOpts, rpc.WithTLSConfig(manager.opts.tlsConfig))
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

		var outerError error
		for attempt := 0; attempt < 3; attempt++ {
			robotClient, err := client.New(ctx, config.Address, logger, client.WithDialOptions(dialOpts...))
			if err != nil {
				if errors.Is(err, rpc.ErrInsecureWithCredentials) {
					if manager.opts.fromCommand {
						err = errors.New("must use -allow-insecure-creds flag to connect to a non-TLS secured robot")
					} else {
						err = errors.New("must use Config.AllowInsecureCreds to connect to a non-TLS secured robot")
					}
					return errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
				}
				outerError = errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
			} else {
				configCopy := config
				manager.addRemote(newRemoteRobot(robotClient, configCopy), configCopy)
				outerError = nil
				break
			}
		}
		if outerError != nil {
			return outerError
		}
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
	part, ok := manager.remotes[name]
	if ok {
		return part, true
	}
	for _, remote := range manager.remotes {
		part, ok := remote.RemoteByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

func (manager *resourceManager) UpdateConfig(ctx context.Context,
	added *config.Config,
	modified *config.ModifiedConfigDiff,
	logger golog.Logger,
	robot *draftRobot) (PartsMergeResult, error) {
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
	replacedServices, err := manager.updateServices(ctx, added.Services, modified.Services, robot)
	leftovers.ReplacedServicesComponents = replacedServices
	if err != nil {
		return leftovers, err
	}
	if err := manager.updateComponentsGraph(added.Components, modified.Components, logger, robot); err != nil {
		return leftovers, err
	}
	replacedComponents, err := manager.updateComponents(ctx, logger, robot)
	if err != nil {
		return leftovers, err
	}
	leftovers.ReplacedServicesComponents = append(leftovers.ReplacedServicesComponents, replacedComponents...)
	return leftovers, nil
}

func (manager *resourceManager) updateProcesses(ctx context.Context,
	addProcesses []pexec.ProcessConfig,
	modifiedProcesses []pexec.ProcessConfig) ([]pexec.ManagedProcess, error) {
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
	logger golog.Logger) ([]*remoteRobot, error) {
	var replacedRemotes []*remoteRobot
	if err := manager.newRemotes(ctx, addedRemotes, logger); err != nil {
		return nil, err
	}
	for _, r := range modifiedRemotes {
		old := manager.remotes[r.Name]
		if err := manager.newRemotes(ctx, []config.Remote{r}, logger); err != nil {
			return replacedRemotes, err
		}
		replacedRemotes = append(replacedRemotes, old)
	}
	return replacedRemotes, nil
}

func (manager *resourceManager) updateServices(ctx context.Context,
	addedServices []config.Service,
	modifiedServices []config.Service,
	robot *draftRobot) ([]interface{}, error) {
	var replacedServices []interface{}
	for _, c := range addedServices {
		// DataManagerService has to be specifically excluded since it's defined in the config but is a default
		// service that we only want to reconfigure rather than reinstantiate with New().
		if c.ResourceName() == datamanager.Name {
			continue
		}
		svc, err := robot.newService(ctx, c)
		if err != nil {
			return nil, err
		}
		manager.addResource(c.ResourceName(), svc)
	}
	for _, c := range modifiedServices {
		if c.ResourceName() == datamanager.Name {
			continue
		}
		svc, err := robot.newService(ctx, c)
		if err != nil {
			return nil, err
		}
		old, ok := manager.resources.Nodes[c.ResourceName()]
		if !ok {
			return nil, errors.Errorf("couldn't find %q service while we are trying to modify it", c.ResourceName())
		}
		oldPart, oldServiceIsReconfigurable := old.(resource.Reconfigurable)
		newPart, newServiceIsReconfigurable := svc.(resource.Reconfigurable)
		switch {
		case oldServiceIsReconfigurable != newServiceIsReconfigurable:
			// this is an indicator of a serious constructor problem
			// for the resource subtype.
			reconfError := errors.Errorf(
				"new type %T is reconfigurable whereas old type %T is not",
				svc, old)
			if oldServiceIsReconfigurable {
				reconfError = errors.Errorf(
					"old type %T is reconfigurable whereas new type %T is not",
					old, svc)
			}
			return nil, reconfError
		case oldServiceIsReconfigurable && newServiceIsReconfigurable:
			// if we are dealing with a reconfigurable resource
			// use the new resource to reconfigure the old one.
			if err := oldPart.Reconfigure(ctx, newPart); err != nil {
				return nil, err
			}
			manager.resources.Nodes[c.ResourceName()] = old
		case !oldServiceIsReconfigurable && !newServiceIsReconfigurable:
			// if we are not dealing with a reconfigurable resource
			// we want to close the old resource and replace it with the
			// new.
			replacedServices = append(replacedServices, old)
			manager.resources.Nodes[c.ResourceName()] = svc
		}
	}
	return replacedServices, nil
}

func (manager *resourceManager) updateComponent(ctx context.Context,
	rName resource.Name,
	conf config.Component,
	old interface{},
	r *draftRobot) ([]interface{}, error) {
	var replacedComponents []interface{}
	obj, canValidate := old.(interface {
		ShouldUpdate(config *config.Component) robot.ShouldUpdateAction
	})
	res := robot.Rebuild
	if canValidate {
		res = obj.ShouldUpdate(&conf)
	}
	switch res {
	case robot.None:
		return replacedComponents, nil
	case robot.Reconfigure:
		sg, err := manager.resources.SubGraphFrom(rName)
		if err != nil {
			return replacedComponents, err
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
				return replacedComponents, err
			}
			wrapper := &resourceUpdateWrapper{
				real:       nil,
				config:     originalConfig,
				isAdded:    true,
				isModified: false,
			}
			manager.resources.Nodes[x] = wrapper
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return nil, err
		}
		oldPart, oldComponentIsReconfigurable := old.(resource.Reconfigurable)
		newPart, newComponentIsReconfigurable := nr.(resource.Reconfigurable)
		switch {
		case oldComponentIsReconfigurable != newComponentIsReconfigurable:
			// this is an indicator of a serious constructor problem
			// for the resource subtype.
			reconfError := errors.Errorf(
				"new type %T is reconfigurable whereas old type %T is not",
				nr, old)
			if oldComponentIsReconfigurable {
				reconfError = errors.Errorf(
					"old type %T is reconfigurable whereas new type %T is not",
					old, nr)
			}
			return replacedComponents, reconfError
		case oldComponentIsReconfigurable && newComponentIsReconfigurable:
			// if we are dealing with a reconfigurable resource
			// use the new resource to reconfigure the old one.
			if err := oldPart.Reconfigure(ctx, newPart); err != nil {
				return replacedComponents, err
			}
			manager.resources.Nodes[rName] = old
		case !oldComponentIsReconfigurable && !newComponentIsReconfigurable:
			// if we are not dealing with a reconfigurable resource
			// we want to close the old resource and replace it with the
			// new.
			replacedComponents = append(replacedComponents, old)
			manager.resources.Nodes[rName] = nr
		}
	case robot.Rebuild:
		sg, err := manager.resources.SubGraphFrom(rName)
		if err != nil {
			return replacedComponents, err
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
				return replacedComponents, err
			}
			wrapper := &resourceUpdateWrapper{
				real:       nil,
				config:     originalConfig,
				isAdded:    true,
				isModified: false,
			}
			manager.resources.Nodes[x] = wrapper
		}
		if err := utils.TryClose(ctx, manager.resources.Nodes[rName]); err != nil {
			return replacedComponents, err
		}
		nr, err := r.newResource(ctx, conf)
		if err != nil {
			return nil, err
		}
		manager.resources.Nodes[rName] = nr
	}
	return replacedComponents, nil
}

func (manager *resourceManager) updateComponents(ctx context.Context,
	logger golog.Logger,
	robot *draftRobot) ([]interface{}, error) {
	var replacedComponents []interface{}
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
				return replacedComponents, err
			}
			manager.resources.Nodes[c] = r
		} else if wrapper.isModified {
			replaced, err := manager.updateComponent(ctx, c, wrapper.config, wrapper.real, robot)
			if err != nil {
				return replacedComponents, err
			}
			replacedComponents = append(replacedComponents, replaced...)
		}
	}
	return replacedComponents, nil
}

func (manager *resourceManager) updateComponentsGraph(addedComponents []config.Component,
	modifiedComponents []config.Component,
	logger golog.Logger,
	robot *draftRobot) error {
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
		// parse the dependency tree and optionnally add/remove components
		for _, dep := range modif.DependsOn {
			var comp resource.Name
			if r := robot.original.config.FindComponent(dep); r != nil {
				comp = r.ResourceName()
			} else if r, ok := manager.resources.FindNodeByName(dep); ok {
				comp = *r
			} else {
				return errors.Errorf("componenent %q depends on non-existent component %q",
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
	partExists := false
	robotPart, ok := manager.resources.Nodes[name]
	if ok {
		return robotPart, nil
	}
	for _, remote := range manager.remotes {
		// only check for part if remote has prefix and the prefix matches the name of remote OR if remote doesn't have a prefix
		if (remote.conf.Prefix && strings.HasPrefix(name.Name, remote.conf.Name)) || !remote.conf.Prefix {
			part, err := remote.ResourceByName(name)
			if err == nil {
				if partExists {
					return nil, errors.Errorf("multiple remote resources with name %q. Change duplicate names to access", name)
				}
				robotPart = part
				partExists = true
			}
		}
	}
	if partExists {
		return robotPart, nil
	}
	return nil, rutils.NewResourceNotFoundError(name)
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses          []pexec.ManagedProcess
	ReplacedRemotes            []*remoteRobot
	ReplacedServicesComponents []interface{}
}

// Process integrates the results into the given manager.
func (result *PartsMergeResult) Process(ctx context.Context, manager *resourceManager) error {
	for _, proc := range result.ReplacedProcesses {
		if replaced, err := manager.processManager.AddProcess(ctx, proc, false); err != nil {
			return err
		} else if replaced != nil {
			return errors.Errorf("unexpected process replacement %v", replaced)
		}
	}
	return nil
}

// MergeAdd merges in the given add manager and returns results for
// later processing.
func (manager *resourceManager) MergeAdd(toAdd *resourceManager) (*PartsMergeResult, error) {
	if len(toAdd.remotes) != 0 {
		if manager.remotes == nil {
			manager.remotes = make(map[string]*remoteRobot, len(toAdd.remotes))
		}
		for k, v := range toAdd.remotes {
			manager.remotes[k] = v
		}
	}

	err := manager.resources.MergeAdd(toAdd.resources)
	if err != nil {
		return nil, err
	}

	var result PartsMergeResult
	if toAdd.processManager != nil {
		// assume manager.processManager is non-nil
		replaced, err := pexec.MergeAddProcessManagers(manager.processManager, toAdd.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	return &result, nil
}

// MergeModify merges in the modified manager and returns results for
// later processing.
func (manager *resourceManager) MergeModify(ctx context.Context, toModify *resourceManager, diff *config.Diff) (*PartsMergeResult, error) {
	var result PartsMergeResult
	if toModify.processManager != nil {
		// assume manager.processManager is non-nil
		// adding also replaces here
		replaced, err := pexec.MergeAddProcessManagers(manager.processManager, toModify.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	// this is the point of no return during reconfiguration
	if len(toModify.remotes) != 0 {
		for k, v := range toModify.remotes {
			old, ok := manager.remotes[k]
			if !ok {
				// should not happen
				continue
			}
			old.replace(ctx, v)
		}
	}
	orderedModify := toModify.resources.ReverseTopologicalSort()
	if len(orderedModify) != 0 {
		for _, k := range orderedModify {
			v := toModify.resources.Nodes[k]
			old, ok := manager.resources.Nodes[k]
			if !ok {
				// should not happen
				continue
			}
			if err := manager.resources.ReplaceNodesParents(k, toModify.resources); err != nil {
				return nil, err
			}
			if v == old {
				// same underlying resource so we can continue
				continue
			}
			oldPart, oldIsReconfigurable := old.(resource.Reconfigurable)
			newPart, newIsReconfigurable := v.(resource.Reconfigurable)
			switch {
			case oldIsReconfigurable != newIsReconfigurable:
				// this is an indicator of a serious constructor problem
				// for the resource subtype.
				reconfError := errors.Errorf(
					"new type %T is reconfigurable whereas old type %T is not",
					v, old)
				if oldIsReconfigurable {
					reconfError = errors.Errorf(
						"old type %T is reconfigurable whereas new type %T is not",
						old, v)
				}
				return nil, reconfError
			case oldIsReconfigurable && newIsReconfigurable:
				// if we are dealing with a reconfigurable resource
				// use the new resource to reconfigure the old one.
				if err := oldPart.Reconfigure(ctx, newPart); err != nil {
					return nil, err
				}
			case !oldIsReconfigurable && !newIsReconfigurable:
				// if we are not dealing with a reconfigurable resource
				// we want to close the old resource and replace it with the
				// new.
				if err := utils.TryClose(ctx, old); err != nil {
					return nil, err
				}
				// Not sure if this is the best approach, here I assume both ressources share the same dependencies
				manager.resources.Nodes[k] = v
			}
		}
	}

	return &result, nil
}

// MergeRemove merges in the removed manager but does no work
// to stop the individual parts.
func (manager *resourceManager) MergeRemove(toRemove *resourceManager) {
	if len(toRemove.remotes) != 0 {
		for k := range toRemove.remotes {
			delete(manager.remotes, k)
		}
	}

	manager.resources.MergeRemove(toRemove.resources)

	if toRemove.processManager != nil {
		// assume manager.processManager is non-nil
		// ignoring result as we will filter out the processes to remove and stop elsewhere
		pexec.MergeRemoveProcessManagers(manager.processManager, toRemove.processManager)
	}
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
		part, ok := manager.remotes[conf.Name]
		if !ok {
			continue
		}
		filtered.addRemote(part, conf)
		delete(manager.remotes, conf.Name)
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
