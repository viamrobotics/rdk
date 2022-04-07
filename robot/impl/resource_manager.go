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
	functions      map[string]struct{}
	resources      *resource.Graph
	processManager pexec.ProcessManager
	opts           resourceManagerOptions
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
		functions:      map[string]struct{}{},
		resources:      resource.NewGraph(),
		processManager: pexec.NewProcessManager(logger),
		opts:           opts,
	}
}

// addRemote adds a remote to the manager.
func (manager *resourceManager) addRemote(r *remoteRobot, c config.Remote) {
	manager.remotes[c.Name] = r
}

// addFunction adds a function to the manager.
func (manager *resourceManager) addFunction(name string) {
	manager.functions[name] = struct{}{}
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

// mergeNamesWithRemotes merges names from the manager itself as well as its
// remotes.
func (manager *resourceManager) mergeNamesWithRemotes(names []string, namesFunc func(remote robot.Robot) []string) []string {
	// use this to filter out seen names and preserve order
	seen := utils.NewStringSet()
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range manager.remotes {
		remoteNames := namesFunc(r)
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

// FunctionNames returns the names of all functions in the manager.
func (manager *resourceManager) FunctionNames() []string {
	names := []string{}
	for k := range manager.functions {
		names = append(names, k)
	}
	return manager.mergeNamesWithRemotes(names, robot.Robot.FunctionNames)
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
	if len(manager.functions) != 0 {
		clonedManager.functions = make(map[string]struct{}, len(manager.functions))
		for k, v := range manager.functions {
			clonedManager.functions[k] = v
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

	for _, f := range config.Functions {
		manager.addFunction(f.Name)
	}

	return nil
}

// processModifiedConfig ingests a given config and constructs all constituent parts.
func (manager *resourceManager) processModifiedConfig(
	ctx context.Context,
	config *config.ModifiedConfigDiff,
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

	for _, f := range config.Functions {
		manager.addFunction(f.Name)
	}

	return nil
}

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
	ReplacedProcesses []pexec.ManagedProcess
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

	if len(toAdd.functions) != 0 {
		if manager.functions == nil {
			manager.functions = make(map[string]struct{}, len(toAdd.functions))
		}
		for k, v := range toAdd.functions {
			manager.functions[k] = v
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

	if len(toRemove.functions) != 0 {
		for k := range toRemove.functions {
			delete(manager.functions, k)
		}
	}
	manager.resources.MergeRemove(toRemove.resources)

	if toRemove.processManager != nil {
		// assume manager.processManager is non-nil
		// ignoring result as we will filter out the processes to remove and stop elsewhere
		pexec.MergeRemoveProcessManagers(manager.processManager, toRemove.processManager)
	}
}

// FilterFromConfig returns a shallow copy of the manager reflecting
// a given config.
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
	}

	for _, conf := range conf.Remotes {
		part, ok := manager.remotes[conf.Name]
		if !ok {
			continue
		}
		filtered.addRemote(part, conf)
	}

	for _, compConf := range conf.Components {
		rName := compConf.ResourceName()
		_, err := manager.ResourceByName(rName)
		if err != nil {
			continue
		}
		// Assuming dependencies will be added later
		filtered.resources.AddNode(rName, manager.resources.Nodes[rName])
	}

	for _, conf := range conf.Services {
		rName := conf.ResourceName()
		_, err := manager.ResourceByName(rName)
		if err != nil {
			continue
		}
		// Assuming dependencies will be added later
		filtered.resources.AddNode(rName, manager.resources.Nodes[rName])
	}

	for _, conf := range conf.Functions {
		_, ok := manager.functions[conf.Name]
		if !ok {
			continue
		}
		filtered.addFunction(conf.Name)
	}

	return filtered, nil
}
