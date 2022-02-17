package robotimpl

import (
	"context"
	"crypto/tls"
	"os"

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
)

// robotParts are the actual parts that make up a robot.
type robotParts struct {
	remotes        map[string]*remoteRobot
	functions      map[string]struct{}
	resources      *resource.Graph
	processManager pexec.ProcessManager
	opts           robotPartsOptions
}

type robotPartsOptions struct {
	debug              bool
	fromCommand        bool
	allowInsecureCreds bool
	tlsConfig          *tls.Config
}

// newRobotParts returns a properly initialized set of parts.
func newRobotParts(
	opts robotPartsOptions,
	logger golog.Logger,
) *robotParts {
	return &robotParts{
		remotes:        map[string]*remoteRobot{},
		functions:      map[string]struct{}{},
		resources:      resource.NewGraph(),
		processManager: pexec.NewProcessManager(logger),
		opts:           opts,
	}
}

// addRemote adds a remote to the parts.
func (parts *robotParts) addRemote(r *remoteRobot, c config.Remote) {
	parts.remotes[c.Name] = r
}

// addFunction adds a function to the parts.
func (parts *robotParts) addFunction(name string) {
	parts.functions[name] = struct{}{}
}

// addResource adds a resource to the parts.
func (parts *robotParts) addResource(name resource.Name, r interface{}) {
	parts.resources.AddNode(name, r)
}

// RemoteNames returns the names of all remotes in the parts.
func (parts *robotParts) RemoteNames() []string {
	names := []string{}
	for k := range parts.remotes {
		names = append(names, k)
	}
	return names
}

// mergeNamesWithRemotes merges names from the parts itself as well as its
// remotes.
func (parts *robotParts) mergeNamesWithRemotes(names []string, namesFunc func(remote robot.Robot) []string) []string {
	// use this to filter out seen names and preserve order
	seen := utils.NewStringSet()
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range parts.remotes {
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

// mergeResourceNamesWithRemotes merges names from the parts itself as well as its
// remotes.
func (parts *robotParts) mergeResourceNamesWithRemotes(names []resource.Name) []resource.Name {
	// use this to filter out seen names and preserve order
	seen := make(map[resource.Name]struct{}, len(parts.resources.Nodes))
	for _, name := range names {
		seen[name] = struct{}{}
	}
	for _, r := range parts.remotes {
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

// FunctionNames returns the names of all functions in the parts.
func (parts *robotParts) FunctionNames() []string {
	names := []string{}
	for k := range parts.functions {
		names = append(names, k)
	}
	return parts.mergeNamesWithRemotes(names, robot.Robot.FunctionNames)
}

// ResourceNames returns the names of all resources in the parts.
func (parts *robotParts) ResourceNames() []resource.Name {
	names := []resource.Name{}
	for k := range parts.resources.Nodes {
		names = append(names, k)
	}
	return parts.mergeResourceNamesWithRemotes(names)
}

// Clone provides a shallow copy of each part.
func (parts *robotParts) Clone() *robotParts {
	var clonedParts robotParts
	if len(parts.remotes) != 0 {
		clonedParts.remotes = make(map[string]*remoteRobot, len(parts.remotes))
		for k, v := range parts.remotes {
			clonedParts.remotes[k] = v
		}
	}
	if len(parts.functions) != 0 {
		clonedParts.functions = make(map[string]struct{}, len(parts.functions))
		for k, v := range parts.functions {
			clonedParts.functions[k] = v
		}
	}
	if len(parts.resources.Nodes) != 0 {
		clonedParts.resources = parts.resources.Clone()
	}
	if parts.processManager != nil {
		clonedParts.processManager = parts.processManager.Clone()
	}
	return &clonedParts
}

// Close attempts to close/stop all parts.
func (parts *robotParts) Close(ctx context.Context) error {
	var allErrs error
	if err := parts.processManager.Stop(); err != nil {
		allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error stopping process manager"))
	}

	for _, x := range parts.remotes {
		if err := utils.TryClose(ctx, x); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing remote"))
		}
	}

	order := parts.resources.TopologicalSort()
	for _, x := range order {
		if err := utils.TryClose(ctx, parts.resources.Nodes[x]); err != nil {
			allErrs = multierr.Combine(allErrs, errors.Wrap(err, "error closing resource"))
		}
	}

	return allErrs
}

// processConfig ingests a given config and constructs all constituent parts.
func (parts *robotParts) processConfig(
	ctx context.Context,
	config *config.Config,
	robot *localRobot,
	logger golog.Logger,
) error {
	if err := parts.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := parts.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := parts.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	if err := parts.newServices(ctx, config.Services, robot); err != nil {
		return err
	}

	for _, f := range config.Functions {
		parts.addFunction(f.Name)
	}

	return nil
}

// processModifiedConfig ingests a given config and constructs all constituent parts.
func (parts *robotParts) processModifiedConfig(
	ctx context.Context,
	config *config.ModifiedConfigDiff,
	robot *localRobot,
	logger golog.Logger,
) error {
	if err := parts.newProcesses(ctx, config.Processes); err != nil {
		return err
	}

	if err := parts.newRemotes(ctx, config.Remotes, logger); err != nil {
		return err
	}

	if err := parts.newComponents(ctx, config.Components, robot); err != nil {
		return err
	}

	if err := parts.newServices(ctx, config.Services, robot); err != nil {
		return err
	}

	for _, f := range config.Functions {
		parts.addFunction(f.Name)
	}

	return nil
}

// newProcesses constructs all processes defined.
func (parts *robotParts) newProcesses(ctx context.Context, processes []pexec.ProcessConfig) error {
	for _, procConf := range processes {
		// In an AppImage execve() is meant to be hooked to swap out the AppImage's libraries and the system ones.
		// Go doesn't use libc's execve() though, so the hooks fail and trying to exec binaries outside the AppImage can fail.
		// We work around this by execing through a bash shell (included in the AppImage) which then gets hooked properly.
		_, isAppImage := os.LookupEnv("APPIMAGE")
		if isAppImage {
			procConf.Args = []string{"-c", shellescape.QuoteCommand(append([]string{procConf.Name}, procConf.Args...))}
			procConf.Name = "bash"
		}

		if _, err := parts.processManager.AddProcessFromConfig(ctx, procConf); err != nil {
			return err
		}
	}
	return parts.processManager.Start(ctx)
}

// newRemotes constructs all remotes defined and integrates their parts in.
func (parts *robotParts) newRemotes(ctx context.Context, remotes []config.Remote, logger golog.Logger) error {
	for _, config := range remotes {
		var dialOpts []rpc.DialOption
		if parts.opts.debug {
			dialOpts = append(dialOpts, rpc.WithDialDebug())
		}
		if config.Insecure {
			dialOpts = append(dialOpts, rpc.WithInsecure())
		}
		if parts.opts.allowInsecureCreds {
			dialOpts = append(dialOpts, rpc.WithAllowInsecureWithCredentialsDowngrade())
		}
		if parts.opts.tlsConfig != nil {
			dialOpts = append(dialOpts, rpc.WithTLSConfig(parts.opts.tlsConfig))
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
		} else {
			dialOpts = append(dialOpts, rpc.WithWebRTCOptions(rpc.DialWebRTCOptions{
				Config: &rpc.DefaultWebRTCConfiguration,
			}))
		}

		robotClient, err := client.New(ctx, config.Address, logger, client.WithDialOptions(dialOpts...))
		if err != nil {
			if errors.Is(err, rpc.ErrInsecureWithCredentials) {
				if parts.opts.fromCommand {
					err = errors.New("must use -allow-insecure-creds flag to connect to a non-TLS secured robot")
				} else {
					err = errors.New("must use Config.AllowInsecureCreds to connect to a non-TLS secured robot")
				}
			}
			return errors.Wrapf(err, "couldn't connect to robot remote (%s)", config.Address)
		}

		configCopy := config
		parts.addRemote(newRemoteRobot(robotClient, configCopy), configCopy)
	}
	return nil
}

// newComponents constructs all components defined.
func (parts *robotParts) newComponents(ctx context.Context, components []config.Component, robot *localRobot) error {
	for _, c := range components {
		r, err := robot.newResource(ctx, c)
		if err != nil {
			return err
		}
		rName := c.ResourceName()
		parts.addResource(rName, r)
		for _, dep := range c.DependsOn {
			if comp := robot.config.FindComponent(dep); comp != nil {
				if err := parts.resources.AddChildren(rName, comp.ResourceName()); err != nil {
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
func (parts *robotParts) newServices(ctx context.Context, services []config.Service, r *localRobot) error {
	for _, c := range services {
		svc, err := r.newService(ctx, c)
		if err != nil {
			return err
		}
		parts.addResource(c.ResourceName(), svc)
	}

	return nil
}

// RemoteByName returns the given remote robot by name, if it exists;
// returns nil otherwise.
func (parts *robotParts) RemoteByName(name string) (robot.Robot, bool) {
	part, ok := parts.remotes[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.RemoteByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// ResourceByName returns the given resource by fully qualified name, if it exists;
// returns nil otherwise.
func (parts *robotParts) ResourceByName(name resource.Name) (interface{}, bool) {
	part, ok := parts.resources.Nodes[name]
	if ok {
		return part, true
	}
	for _, remote := range parts.remotes {
		part, ok := remote.ResourceByName(name)
		if ok {
			return part, true
		}
	}
	return nil, false
}

// PartsMergeResult is the result of merging in parts together.
type PartsMergeResult struct {
	ReplacedProcesses []pexec.ManagedProcess
}

// Process integrates the results into the given parts.
func (result *PartsMergeResult) Process(ctx context.Context, parts *robotParts) error {
	for _, proc := range result.ReplacedProcesses {
		if replaced, err := parts.processManager.AddProcess(ctx, proc, false); err != nil {
			return err
		} else if replaced != nil {
			return errors.Errorf("unexpected process replacement %v", replaced)
		}
	}
	return nil
}

// MergeAdd merges in the given added parts and returns results for
// later processing.
func (parts *robotParts) MergeAdd(toAdd *robotParts) (*PartsMergeResult, error) {
	if len(toAdd.remotes) != 0 {
		if parts.remotes == nil {
			parts.remotes = make(map[string]*remoteRobot, len(toAdd.remotes))
		}
		for k, v := range toAdd.remotes {
			parts.remotes[k] = v
		}
	}

	if len(toAdd.functions) != 0 {
		if parts.functions == nil {
			parts.functions = make(map[string]struct{}, len(toAdd.functions))
		}
		for k, v := range toAdd.functions {
			parts.functions[k] = v
		}
	}

	err := parts.resources.MergeAdd(toAdd.resources)
	if err != nil {
		return nil, err
	}

	var result PartsMergeResult
	if toAdd.processManager != nil {
		// assume parts.processManager is non-nil
		replaced, err := pexec.MergeAddProcessManagers(parts.processManager, toAdd.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	return &result, nil
}

// MergeModify merges in the given modified parts and returns results for
// later processing.
func (parts *robotParts) MergeModify(ctx context.Context, toModify *robotParts, diff *config.Diff) (*PartsMergeResult, error) {
	var result PartsMergeResult
	if toModify.processManager != nil {
		// assume parts.processManager is non-nil
		// adding also replaces here
		replaced, err := pexec.MergeAddProcessManagers(parts.processManager, toModify.processManager)
		if err != nil {
			return nil, err
		}
		result.ReplacedProcesses = replaced
	}

	// this is the point of no return during reconfiguration

	if len(toModify.remotes) != 0 {
		for k, v := range toModify.remotes {
			old, ok := parts.remotes[k]
			if !ok {
				// should not happen
				continue
			}
			old.replace(ctx, v)
		}
	}

	if len(toModify.resources.Nodes) != 0 {
		for k, v := range toModify.resources.Nodes {
			old, ok := parts.resources.Nodes[k]
			if !ok {
				// should not happen
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
				parts.resources.Nodes[k] = v
			}
		}
	}

	return &result, nil
}

// MergeRemove merges in the given removed parts but does no work
// to stop the individual parts.
func (parts *robotParts) MergeRemove(toRemove *robotParts) {
	if len(toRemove.remotes) != 0 {
		for k := range toRemove.remotes {
			delete(parts.remotes, k)
		}
	}

	if len(toRemove.functions) != 0 {
		for k := range toRemove.functions {
			delete(parts.functions, k)
		}
	}
	parts.resources.MergeRemove(toRemove.resources)

	if toRemove.processManager != nil {
		// assume parts.processManager is non-nil
		// ignoring result as we will filter out the processes to remove and stop elsewhere
		pexec.MergeRemoveProcessManagers(parts.processManager, toRemove.processManager)
	}
}

// FilterFromConfig returns a shallow copy of the parts reflecting
// a given config.
func (parts *robotParts) FilterFromConfig(ctx context.Context, conf *config.Config, logger golog.Logger) (*robotParts, error) {
	filtered := newRobotParts(parts.opts, logger)

	for _, conf := range conf.Processes {
		proc, ok := parts.processManager.ProcessByID(conf.ID)
		if !ok {
			continue
		}
		if _, err := filtered.processManager.AddProcess(ctx, proc, false); err != nil {
			return nil, err
		}
	}

	for _, conf := range conf.Remotes {
		part, ok := parts.remotes[conf.Name]
		if !ok {
			continue
		}
		filtered.addRemote(part, conf)
	}

	for _, compConf := range conf.Components {
		rName := compConf.ResourceName()
		_, ok := parts.ResourceByName(rName)
		if !ok {
			continue
		}
		if err := filtered.resources.MergeNode(rName, parts.resources); err != nil {
			return nil, err
		}
	}

	for _, conf := range conf.Services {
		rName := conf.ResourceName()
		_, ok := parts.ResourceByName(rName)
		if !ok {
			continue
		}
		if err := filtered.resources.MergeNode(rName, parts.resources); err != nil {
			return nil, err
		}
	}

	for _, conf := range conf.Functions {
		_, ok := parts.functions[conf.Name]
		if !ok {
			continue
		}
		filtered.addFunction(conf.Name)
	}

	return filtered, nil
}
