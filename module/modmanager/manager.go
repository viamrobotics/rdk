// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/robot/packages"
	rutils "go.viam.com/rdk/utils"
)

var (
	validateConfigTimeout       = 5 * time.Second
	errMessageExitStatus143     = "exit status 143"
	logLevelArgumentTemplate    = "--log-level=%s"
	errModularResourcesDisabled = errors.New("modular resources disabled in untrusted environment")
	// name of the folder under the viamHomeDir that holds all the folders for the module data
	// ex: /home/walle/.viam/module-data/<cloud-robot-id>/<module-name>
	parentModuleDataFolderName = "module-data"
)

// NewManager returns a Manager.
func NewManager(
	ctx context.Context, parentAddrs config.ParentSockAddrs, logger logging.Logger, options modmanageroptions.Options,
) (*Manager, error) {
	var err error
	parentAddrs.UnixAddr, err = rutils.CleanWindowsSocketPath(runtime.GOOS, parentAddrs.UnixAddr)
	if err != nil {
		return nil, err
	}
	restartCtx, restartCtxCancel := context.WithCancel(ctx)
	ret := &Manager{
		logger:                  logger.Sublogger("modmanager"),
		modules:                 moduleMap{},
		parentAddrs:             parentAddrs,
		rMap:                    resourceModuleMap{},
		untrustedEnv:            options.UntrustedEnv,
		viamHomeDir:             options.ViamHomeDir,
		moduleDataParentDir:     getModuleDataParentDirectory(options),
		handleOrphanedResources: options.HandleOrphanedResources,
		restartCtx:              restartCtx,
		restartCtxCancel:        restartCtxCancel,
		packagesDir:             options.PackagesDir,
		ftdc:                    options.FTDC,
		modPeerConnTracker:      options.ModPeerConnTracker,
	}
	return ret, nil
}

type addedResource struct {
	conf resource.Config
	deps []string
}

// moduleMap is a typesafe wrapper for a sync.Map holding string keys and *module values.
type moduleMap struct {
	items sync.Map
}

func (mmap *moduleMap) Store(name string, mod *module) { mmap.items.Store(name, mod) }
func (mmap *moduleMap) Delete(name string)             { mmap.items.Delete(name) }

func (mmap *moduleMap) Load(name string) (*module, bool) {
	value, ok := mmap.items.Load(name)
	if value == nil {
		return nil, ok
	}
	return value.(*module), ok
}

func (mmap *moduleMap) Range(f func(name string, mod *module) bool) {
	mmap.items.Range(func(key, value any) bool {
		return f(key.(string), value.(*module))
	})
}

// resourceModuleMap is a typesafe wrapper for a sync.Map holding resource.Name keys and
// *module values.
type resourceModuleMap struct {
	items sync.Map
}

func (rmap *resourceModuleMap) Store(name resource.Name, mod *module) { rmap.items.Store(name, mod) }
func (rmap *resourceModuleMap) Delete(name resource.Name)             { rmap.items.Delete(name) }

func (rmap *resourceModuleMap) Load(name resource.Name) (*module, bool) {
	value, ok := rmap.items.Load(name)
	if value == nil {
		return nil, ok
	}
	return value.(*module), ok
}

func (rmap *resourceModuleMap) Range(f func(name resource.Name, mod *module) bool) {
	rmap.items.Range(func(key, value any) bool {
		return f(key.(resource.Name), value.(*module))
	})
}

// Manager is the root structure for the module system.
type Manager struct {
	// mu (mostly) coordinates API methods that modify the `modules` map. Specifically,
	// `AddResource` can be called concurrently during a reconfigure. But `RemoveResource` or
	// resources being restarted after a module crash are given exclusive access.
	//
	// mu additionally is used for exclusive access when `Add`ing modules (as opposed to resources),
	// `Reconfigure`ing modules, `Remove`ing modules and `Close`ing the `Manager`.
	mu sync.RWMutex

	logger       logging.Logger
	modules      moduleMap
	parentAddrs  config.ParentSockAddrs
	rMap         resourceModuleMap
	untrustedEnv bool
	// viamHomeDir is the absolute path to the viam home directory. Ex: /home/walle/.viam
	// `viamHomeDir` may only be the empty string in testing
	viamHomeDir string
	// packagesDir is the PackagesPath from a config.Config. It's used for resolving paths for local tarball modules.
	packagesDir string
	// moduleDataParentDir is the absolute path to the current robots module data directory.
	// Ex: /home/walle/.viam/module-data/<cloud-robot-id>
	// it is empty if the modmanageroptions.Options.viamHomeDir was empty
	moduleDataParentDir     string
	handleOrphanedResources func(ctx context.Context, rNames []resource.Name)
	restartCtx              context.Context
	restartCtxCancel        context.CancelFunc
	ftdc                    *ftdc.FTDC

	// modPeerConnTracker must be updated as modules create/destroy any underlying WebRTC
	// PeerConnections.
	modPeerConnTracker *rdkgrpc.ModPeerConnTracker
}

// Close terminates module connections and processes.
func (mgr *Manager) Close(ctx context.Context) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.restartCtxCancel != nil {
		mgr.restartCtxCancel()
	}
	var err error
	mgr.modules.Range(func(_ string, mod *module) bool {
		err = multierr.Combine(err, mgr.closeModule(mod, false))
		return true
	})
	return err
}

// Kill will kill all processes in the module's process group.
// This is best effort as we do not have a lock during this
// function. Taking the lock will mean that we may be blocked,
// and we do not want to be blocked.
func (mgr *Manager) Kill() {
	if mgr.restartCtxCancel != nil {
		mgr.restartCtxCancel()
	}
	// sync.Map's Range does not block other methods on the map;
	// even f itself may call any method on the map.
	mgr.modules.Range(func(_ string, mod *module) bool {
		mod.killProcessGroup()
		return true
	})
}

// Handles returns all the models for each module registered.
func (mgr *Manager) Handles() map[string]modlib.HandlerMap {
	res := map[string]modlib.HandlerMap{}

	mgr.modules.Range(func(n string, m *module) bool {
		res[n] = m.handles
		return true
	})

	return res
}

// An allowed list of specific namespaces that are allowed to run in an untrusted environment.
// We want to allow running all of our official modules even in an untrusted environment.
var allowedModulesNamespaces = map[string]bool{
	"viam": true,
}

// Checks if the modules added in an untrusted environment are Viam modules
// and returns `true` and a list of their configs if any exist in the passed-in slice.
func checkIfAllowed(confs ...config.Module) (
	allowed bool /*false*/, newConfs []config.Module,
) {
	for _, conf := range confs {
		parts := strings.Split(conf.ModuleID, ":")
		if len(parts) == 0 {
			continue
		}
		namespace := parts[0]
		if ok := allowedModulesNamespaces[namespace]; ok {
			allowed = true
			newConfs = append(newConfs, conf)
		}
	}
	return allowed, newConfs
}

// Add adds and starts a new resource modules for each given module configuration.
//
// Each module configuration should have a unique name - if duplicate names are detected,
// then only the first duplicate instance will be processed and the rest will be ignored.
func (mgr *Manager) Add(ctx context.Context, confs ...config.Module) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.untrustedEnv {
		allowed, newConfs := checkIfAllowed(confs...)
		if !allowed {
			return errModularResourcesDisabled
		}
		// overwrite with just the modules we've allowed
		confs = newConfs
		mgr.logger.CWarnw(
			ctx, "Running in an untrusted environment; will only add some modules", "modules",
			confs)
	}

	var (
		wg   sync.WaitGroup
		errs = make([]error, len(confs))
		seen = make(map[string]struct{}, len(confs))
	)
	for i, conf := range confs {
		if _, dupe := seen[conf.Name]; dupe {
			continue
		}
		seen[conf.Name] = struct{}{}

		// The config was already validated, but we must check again before attempting to add.
		if err := conf.Validate(""); err != nil {
			mgr.logger.CErrorw(ctx, "Module config validation error; skipping", "module", conf.Name, "error", err)
			errs[i] = err
			continue
		}

		// setup valid, new modules in parallel
		wg.Add(1)
		go func(i int, conf config.Module) {
			defer wg.Done()
			moduleLogger := mgr.logger.Sublogger(conf.Name)

			moduleLogger.CInfow(ctx, "Now adding module", "module", conf.Name)
			err := mgr.add(ctx, conf, moduleLogger)
			if err != nil {
				moduleLogger.CErrorw(ctx, "Error adding module", "module", conf.Name, "error", err)
				errs[i] = err
				return
			}
		}(i, conf)
	}
	wg.Wait()

	combinedErr := multierr.Combine(errs...)
	if combinedErr == nil {
		var addedModNames []string
		for modName := range seen {
			addedModNames = append(addedModNames, modName)
		}
		mgr.logger.CInfow(ctx, "Modules successfully added", "modules", addedModNames)
	}
	return combinedErr
}

func (mgr *Manager) add(ctx context.Context, conf config.Module, moduleLogger logging.Logger) error {
	_, exists := mgr.modules.Load(conf.Name)
	if exists {
		// Keeping this as a manager logger since it is dealing with manager behavior
		mgr.logger.CWarnw(ctx, "Not adding module that already exists", "module", conf.Name)
		return nil
	}

	exists, existingName := mgr.execPathAlreadyExists(&conf)
	if exists {
		return errors.Errorf("An existing module %s already exists with the same executable path as module %s", existingName, conf.Name)
	}

	var moduleDataDir string
	// only set the module data directory if the parent dir is present (which it might not be during tests)
	if mgr.moduleDataParentDir != "" {
		var err error
		// TODO: why isn't conf.Name being sanitized like PackageConfig.SanitizedName?
		moduleDataDir, err = rutils.SafeJoinDir(mgr.moduleDataParentDir, conf.Name)
		if err != nil {
			return err
		}
	}

	mod := &module{
		cfg:       conf,
		dataDir:   moduleDataDir,
		resources: map[resource.Name]*addedResource{},
		logger:    moduleLogger,
		ftdc:      mgr.ftdc,
	}

	if err := mgr.startModule(ctx, mod); err != nil {
		return err
	}
	return nil
}

// return appropriate parentaddr for module (select tcp or unix).
func (mgr *Manager) parentAddr(mod *module) string {
	if mod.tcpMode() {
		return mgr.parentAddrs.TCPAddr
	}
	return mgr.parentAddrs.UnixAddr
}

func (mgr *Manager) startModuleProcess(mod *module, oue pexec.UnexpectedExitHandler) error {
	return mod.startProcess(
		mgr.restartCtx,
		mgr.parentAddr(mod),
		oue,
		mgr.viamHomeDir,
		mgr.packagesDir,
	)
}

func (mgr *Manager) startModule(ctx context.Context, mod *module) error {
	var success bool
	defer func() {
		if !success {
			mod.cleanupAfterStartupFailure()
		}
	}()

	// create the module's data directory
	if mod.dataDir != "" {
		mod.logger.Debugf("Creating data directory %q for module %q", mod.dataDir, mod.cfg.Name)
		if err := os.MkdirAll(mod.dataDir, 0o750); err != nil {
			return errors.WithMessage(err, "error while creating data directory for module "+mod.cfg.Name)
		}
	}

	cleanup := rutils.SlowLogger(
		ctx, "Waiting for module to complete startup and registration", "module", mod.cfg.Name, mod.logger)
	defer cleanup()

	var moduleRestartCtx context.Context
	moduleRestartCtx, mod.restartCancel = context.WithCancel(mgr.restartCtx)
	if err := mgr.startModuleProcess(mod, mgr.newOnUnexpectedExitHandler(moduleRestartCtx, mod)); err != nil {
		return errors.WithMessage(err, "error while starting module "+mod.cfg.Name)
	}

	// Does a gRPC dial. Sets up a SharedConn with a PeerConnection that is not yet connected.
	if err := mod.dial(); err != nil {
		return errors.WithMessage(err, "error while dialing module "+mod.cfg.Name)
	}

	// Sends a ReadyRequest and waits on a ReadyResponse. The PeerConnection will async connect
	// after this, so long as the module supports it.
	if err := mod.checkReady(ctx, mgr.parentAddr(mod)); err != nil {
		return errors.WithMessage(err, "error while waiting for module to be ready "+mod.cfg.Name)
	}

	if pc := mod.sharedConn.PeerConn(); mgr.modPeerConnTracker != nil && pc != nil {
		mgr.modPeerConnTracker.Add(mod.cfg.Name, pc)
	}

	mod.registerResourceModels(mgr)
	mgr.modules.Store(mod.cfg.Name, mod)
	mod.logger.Infow("Module successfully added", "module", mod.cfg.Name)
	success = true
	return nil
}

// Reconfigure reconfigures an existing resource module and returns the names of resources previously
// handled by the module.
func (mgr *Manager) Reconfigure(ctx context.Context, conf config.Module) ([]resource.Name, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, exists := mgr.modules.Load(conf.Name)
	if !exists {
		return nil, errors.Errorf("cannot reconfigure module %s as it does not exist", conf.Name)
	}

	exists, existingName := mgr.execPathAlreadyExists(&conf)
	if exists {
		return nil, errors.Errorf("An existing module %s already exists with the same executable path as module %s", existingName, conf.Name)
	}

	handledResources := mod.resources
	var handledResourceNames []resource.Name
	var handledResourceNameStrings []string
	for name := range handledResources {
		handledResourceNames = append(handledResourceNames, name)
		handledResourceNameStrings = append(handledResourceNameStrings, name.String())
	}

	mod.logger.CInfow(ctx, "Module configuration changed. Stopping the existing module process", "module", conf.Name)

	if err := mgr.closeModule(mod, true); err != nil {
		// If removal fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	mod.cfg = conf
	mod.resources = map[resource.Name]*addedResource{}

	mod.logger.CInfow(ctx, "Existing module process stopped. Starting new module process", "module", conf.Name)

	if err := mgr.startModule(ctx, mod); err != nil {
		// If re-addition fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	mod.logger.CInfow(ctx, "New module process is running and responding to gRPC requests", "module",
		mod.cfg.Name, "module address", mod.addr)

	mod.logger.CInfow(ctx, "Resources handled by reconfigured module will be re-added to new module process",
		"module", mod.cfg.Name, "resources", handledResourceNameStrings)
	return handledResourceNames, nil
}

// Remove removes and stops an existing resource module and returns the names of
// now orphaned resources.
func (mgr *Manager) Remove(modName string) ([]resource.Name, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, exists := mgr.modules.Load(modName)
	if !exists {
		return nil, errors.Errorf("cannot remove module %s as it does not exist", modName)
	}

	mgr.logger.Infow("Now removing module", "module", modName)

	handledResources := mod.resources

	mod.pendingRemoval = true
	mod.restartCancel()

	// If module handles no resources, remove it now.
	if len(handledResources) == 0 {
		return nil, mgr.closeModule(mod, false)
	}

	// Otherwise return the list of resources that need to be closed before the
	// module can be cleanly removed.
	var orphanedResourceNames []resource.Name
	var orphanedResourceNameStrings []string
	for name := range handledResources {
		orphanedResourceNames = append(orphanedResourceNames, name)
		orphanedResourceNameStrings = append(orphanedResourceNameStrings, name.String())
	}
	mgr.logger.Infow("Resources handled by removed module will be removed",
		"module", mod.cfg.Name, "resources", orphanedResourceNameStrings)
	return orphanedResourceNames, nil
}

// closeModule attempts to cleanly shut down the module process. It does not wait on module recovery processes,
// as they are running outside code and may have unexpected behavior.
func (mgr *Manager) closeModule(mod *module, reconfigure bool) error {
	// resource manager should've removed these cleanly if this isn't a reconfigure
	if !reconfigure && len(mod.resources) != 0 {
		mod.logger.Warnw("Forcing removal of module with active resources", "module", mod.cfg.Name)
	}

	cleanup := rutils.SlowLogger(
		context.Background(), "Waiting for module to complete shutdown", "module", mod.cfg.Name, mod.logger)
	defer cleanup()

	// Remove all resources associated with the module. Only allow 20s across all removals.
	// Stopping the module process can take up to 10s, and we want closure of a module to
	// take <= 30s.
	resourceRemovalCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for res := range mod.resources {
		_, err := mod.client.RemoveResource(resourceRemovalCtx, &pb.RemoveResourceRequest{Name: res.String()})
		if err != nil {
			mod.logger.Errorw("Error removing resource", "module", mod.cfg.Name, "resource", res.Name, "error", err)
		} else {
			mod.logger.Infow("Successfully removed resource from module", "module", mod.cfg.Name, "resource", res.Name)
		}
	}

	if err := mod.stopProcess(); err != nil {
		return errors.WithMessage(err, "error while stopping module "+mod.cfg.Name)
	}

	if mgr.modPeerConnTracker != nil {
		mgr.modPeerConnTracker.Remove(mod.cfg.Name)
	}
	if err := mod.sharedConn.Close(); err != nil {
		mod.logger.Warnw("Error closing connection to module", "error", err)
	}

	mod.deregisterResourceModels()

	for r, m := range mgr.rMap.Range {
		if m == mod {
			mgr.rMap.Delete(r)
		}
	}
	mgr.modules.Delete(mod.cfg.Name)

	mod.logger.Infow("Module successfully closed", "module", mod.cfg.Name)
	return nil
}

// AddResource tells a component module to configure a new component.
func (mgr *Manager) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.addResource(ctx, conf, deps)
}

func (mgr *Manager) addResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	mod, ok := mgr.getModule(conf)
	if !ok {
		return nil, errors.Errorf("no active module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	mod.logger.CInfow(ctx, "Adding resource to module", "resource", conf.Name, "module", mod.cfg.Name)

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	_, err = mod.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}
	mgr.rMap.Store(conf.ResourceName(), mod)

	mod.resourcesMu.Lock()
	defer mod.resourcesMu.Unlock()
	mod.resources[conf.ResourceName()] = &addedResource{conf, deps}

	apiInfo, ok := resource.LookupGenericAPIRegistration(conf.API)
	if !ok || apiInfo.RPCClient == nil {
		mod.logger.CWarnw(ctx, "No built-in grpc client for modular resource", "resource", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), &mod.sharedConn), nil
	}

	return apiInfo.RPCClient(ctx, &mod.sharedConn, "", conf.ResourceName(), mgr.logger)
}

// ReconfigureResource updates/reconfigures a modular component with a new configuration.
func (mgr *Manager) ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error {
	mod, ok := mgr.getModule(conf)
	if !ok {
		return errors.Errorf("no module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	mod.logger.CInfow(ctx, "Reconfiguring resource for module", "resource", conf.Name, "module", mod.cfg.Name)

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return err
	}
	_, err = mod.client.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return err
	}

	mod.resourcesMu.Lock()
	defer mod.resourcesMu.Unlock()
	mod.resources[conf.ResourceName()] = &addedResource{conf, deps}

	return nil
}

// Configs returns a slice of config.Module representing the currently managed
// modules.
func (mgr *Manager) Configs() []config.Module {
	var configs []config.Module
	mgr.modules.Range(func(_ string, mod *module) bool {
		configs = append(configs, mod.cfg)
		return true
	})
	return configs
}

// AllModels returns a slice of resource.ModuleModel representing the available models
// from the currently managed modules.
func (mgr *Manager) AllModels() []resource.ModuleModel {
	moduleTypes := map[string]config.ModuleType{}
	models := []resource.ModuleModel{}
	for _, moduleConfig := range mgr.Configs() {
		moduleName := moduleConfig.Name
		moduleTypes[moduleName] = moduleConfig.Type
	}
	for moduleName, handleMap := range mgr.Handles() {
		for api, handle := range handleMap {
			for _, model := range handle {
				modelModel := resource.ModuleModel{
					ModuleName: moduleName, Model: model, API: api.API,
					FromLocalModule: moduleTypes[moduleName] == config.ModuleTypeLocal,
				}
				models = append(models, modelModel)
			}
		}
	}
	return models
}

// Provides returns true if a component/service config WOULD be handled by a module.
func (mgr *Manager) Provides(conf resource.Config) bool {
	_, ok := mgr.getModule(conf)
	return ok
}

// IsModularResource returns true if an existing resource IS handled by a module.
func (mgr *Manager) IsModularResource(name resource.Name) bool {
	_, ok := mgr.rMap.Load(name)
	return ok
}

// RemoveResource requests the removal of a resource from a module.
func (mgr *Manager) RemoveResource(ctx context.Context, name resource.Name) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, ok := mgr.rMap.Load(name)
	if !ok {
		return errors.Errorf("resource %+v not found in module", name)
	}

	mod.logger.CInfow(ctx, "Removing resource for module", "resource", name.String(), "module", mod.cfg.Name)

	mgr.rMap.Delete(name)
	delete(mod.resources, name)
	_, err := mod.client.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: name.String()})
	if err != nil && !errors.Is(err, rdkgrpc.ErrNotConnected) {
		return err
	}

	// if the module is marked for removal, actually remove it when the final resource is closed
	if mod.pendingRemoval && len(mod.resources) == 0 {
		return multierr.Combine(err, mgr.closeModule(mod, false))
	}
	return nil
}

// ValidateConfig determines whether the given config is valid and returns its implicit
// required and optional dependencies.
func (mgr *Manager) ValidateConfig(ctx context.Context, conf resource.Config) ([]string, []string, error) {
	mod, ok := mgr.getModule(conf)
	if !ok {
		return nil, nil,
			errors.Errorf("no module registered to serve resource api %s and model %s",
				conf.API, conf.Model)
	}

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, nil, err
	}

	// Override context with new timeout.
	var cancel func()
	ctx, cancel = context.WithTimeout(ctx, validateConfigTimeout)
	defer cancel()

	resp, err := mod.client.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: confProto})
	// Swallow "Unimplemented" gRPC errors from modules that lack ValidateConfig
	// receiving logic.
	if err != nil && status.Code(err) == codes.Unimplemented {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	// RSDK-12124: Ignore any dependency that looks like the user is trying to depend on the
	// framesystem. That can be done through framesystem.FromDependencies in all golang
	// modular resources, but users may think it's a required return-value from Validate.
	var requiredImplicitDeps, optionalImplicitDeps []string
	for _, dep := range resp.Dependencies {
		switch dep {
		case "framesystem", "$framesystem", framesystem.PublicServiceName.String():
			continue
		default:
			requiredImplicitDeps = append(requiredImplicitDeps, dep)
		}
	}
	for _, optionalDep := range resp.OptionalDependencies {
		switch optionalDep {
		case "framesystem", "$framesystem", framesystem.PublicServiceName.String():
			continue
		default:
			optionalImplicitDeps = append(optionalImplicitDeps, optionalDep)
		}
	}

	return requiredImplicitDeps, optionalImplicitDeps, nil
}

// ResolveImplicitDependenciesInConfig mutates the passed in diff to add modular implicit dependencies to added
// and modified resources.
func (mgr *Manager) ResolveImplicitDependenciesInConfig(ctx context.Context, conf *config.Diff) error {
	// If something was added or modified, go through components and services in
	// diff.Added and diff.Modified, call Validate on all those that are modularized,
	// and store implicit dependencies.
	validateModularResources := func(confs []resource.Config) {
		for i, c := range confs {
			if mgr.Provides(c) {
				implicitRequiredDeps, implicitOptionalDeps, err := mgr.ValidateConfig(ctx, c)
				if err != nil {
					mgr.logger.CErrorw(ctx, "Modular config validation error found in resource: "+c.Name, "error", err)
					continue
				}

				// Modify resource config to add its implicit required and optional dependencies.
				confs[i].ImplicitDependsOn = implicitRequiredDeps
				confs[i].ImplicitOptionalDependsOn = implicitOptionalDeps
			}
		}
	}
	if conf.Added != nil {
		validateModularResources(conf.Added.Components)
		validateModularResources(conf.Added.Services)
	}
	if conf.Modified != nil {
		validateModularResources(conf.Modified.Components)
		validateModularResources(conf.Modified.Services)
	}
	return nil
}

func (mgr *Manager) getModule(conf resource.Config) (foundMod *module, exists bool) {
	mgr.modules.Range(func(_ string, mod *module) bool {
		var api resource.RPCAPI
		var ok bool
		for a := range mod.handles {
			if a.API == conf.API {
				api = a
				ok = true
				break
			}
		}
		if !ok {
			return true
		}
		for _, model := range mod.handles[api] {
			if conf.Model == model && !mod.pendingRemoval {
				foundMod = mod
				exists = true
				return false
			}
		}
		return true
	})

	return
}

func (mgr *Manager) execPathAlreadyExists(conf *config.Module) (bool, string) {
	var exists bool
	var existingName string
	mgr.modules.Range(func(_ string, m *module) bool {
		if m.cfg.Name != conf.Name && m.cfg.ExePath == conf.ExePath {
			exists = true
			existingName = m.cfg.Name
			return false
		}
		return true
	})
	return exists, existingName
}

// CleanModuleDataDirectory removes unexpected folders and files from the robot's module data directory.
// Modules removed from the robot config (even temporarily) will get pruned here.
func (mgr *Manager) CleanModuleDataDirectory() error {
	if mgr.moduleDataParentDir == "" {
		return errors.New("cannot clean a root level module data directory")
	}
	// Early exit if the moduleDataParentDir has not been created because there is nothing to clean
	if _, err := os.Stat(mgr.moduleDataParentDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	// Absolute path to all dirs that should exist
	expectedDirs := make(map[string]bool)
	mgr.modules.Range(func(_ string, m *module) bool {
		expectedDirs[m.dataDir] = true
		return true
	})
	// If there are no expected directories, we can shortcut and early-exit
	if len(expectedDirs) == 0 {
		mgr.logger.Infow("Removing module data parent directory", "parent dir", mgr.moduleDataParentDir)
		if err := os.RemoveAll(mgr.moduleDataParentDir); err != nil {
			return errors.Wrapf(err, "failed to clean parent module data directory %q", mgr.moduleDataParentDir)
		}
		return nil
	}
	// Scan dataFolder for all existing directories
	existingDirs, err := filepath.Glob(filepath.Join(mgr.moduleDataParentDir, "*"))
	if err != nil {
		return err
	}
	// Delete directories in dataFolder that are not in expectedDirs
	for _, dir := range existingDirs {
		if _, expected := expectedDirs[dir]; !expected {
			// This is already checked in module.add(), however there is no harm in double-checking before recursively deleting directories
			if !strings.HasPrefix(filepath.Clean(dir), filepath.Clean(mgr.moduleDataParentDir)) {
				return errors.Errorf("attempted to delete a module data dir %q which is not in the viam module data directory %q",
					dir, mgr.moduleDataParentDir)
			}
			mgr.logger.Infow("Removing module data directory", "dir", dir)
			if err := os.RemoveAll(dir); err != nil {
				return errors.Wrapf(err, "failed to clean module data directory %q", dir)
			}
		}
	}
	return nil
}

// oueRestartInterval is the interval of time at which an OnUnexpectedExit
// function can attempt to restart the module process. Multiple restart
// attempts will use basic backoff.
var oueRestartInterval = 5 * time.Second

// newOnUnexpectedExitHandler returns the appropriate OnUnexpectedExit function
// for the passed-in module to include in the pexec.ProcessConfig.
func (mgr *Manager) newOnUnexpectedExitHandler(ctx context.Context, mod *module) pexec.UnexpectedExitHandler {
	return func(oueCtx context.Context, exitCode int) (continueAttemptingRestart bool) {
		// Log error immediately, as this is unexpected behavior.
		mod.logger.Errorw(
			"Module has unexpectedly exited.", "module", mod.cfg.Name, "exit_code", exitCode,
		)

		// There are two relevant calls that may race with a crashing module:
		// 1. mgr.Remove, which wants to stop the module and remove it entirely
		// 2. mgr.Reconfigure, which wants to stop the module and replace it with
		//    a new instance using a different configuration.
		// Both lock the manager mutex and then cancel the restart context for the
		// module. To avoid racing we lock the mutex and then check if the context
		// is cancelled, exiting early if so. If we win the race we may restart the
		// module and it will immediately shut down when we release the lock and
		// Remove/Reconfigure runs, which is acceptable.
		locked := false
		lock := func() {
			if !locked {
				mgr.mu.Lock()
				locked = true
			}
		}
		unlock := func() {
			if locked {
				mgr.mu.Unlock()
				locked = false
			}
		}
		defer unlock()

		// Enter a loop trying to restart the module every 5 seconds. If the
		// restart succeeds we return, this goroutine ends, and the management
		// goroutine started by the new module managedProcess handles any future
		// crashes. If the startup fails we kill the new process, its management
		// goroutine returns without doing anything, and we continue to loop until
		// we succeed or our context is cancelled.
		cleanupPerformed := false
		for {
			lock()
			// It's possible the module has been removed or replaced while we were
			// waiting on the lock. Check for a context cancellation to avoid double
			// starting and/or leaking a module process.
			if err := ctx.Err(); err != nil {
				mod.logger.Infow("Restart context canceled, abandoning restart attempt", "err", err)
				return
			}
			if err := oueCtx.Err(); err != nil {
				mod.logger.Infow("pexec context canceled, abandoning restart attempt", "err", err)
				return
			}

			if !cleanupPerformed {
				mod.cleanupAfterCrash(mgr)
				cleanupPerformed = true
			}

			err := mgr.attemptRestart(ctx, mod)
			if err == nil {
				break
			}
			unlock()
			utils.SelectContextOrWait(ctx, oueRestartInterval)
		}
		mod.logger.Infow("Module successfully restarted, re-adding resources", "module", mod.cfg.Name)

		var orphanedResourceNames []resource.Name
		var restoredResourceNamesStr []string
		for name, res := range mod.resources {
			confProto, err := config.ComponentConfigToProto(&res.conf)
			if err != nil {
				mod.logger.Errorw(
					"Failed to re-add resource after module restarted due to config conversion error",
					"module",
					mod.cfg.Name,
					"resource",
					name.String(),
					"error",
					err,
				)
				orphanedResourceNames = append(orphanedResourceNames, name)
				continue
			}
			_, err = mod.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: res.deps})
			if err != nil {
				mod.logger.Errorw(
					"Failed to re-add resource after module restarted",
					"module",
					mod.cfg.Name,
					"resource",
					name.String(),
					"error",
					err,
				)
				orphanedResourceNames = append(orphanedResourceNames, name)

				// At this point, the modmanager is no longer managing this resource and should remove it
				// from its state.
				mgr.rMap.Delete(name)
				delete(mod.resources, name)
				continue
			}
			restoredResourceNamesStr = append(restoredResourceNamesStr, name.String())
		}
		if len(orphanedResourceNames) > 0 && mgr.handleOrphanedResources != nil {
			orphanedResourceNamesStr := make([]string, len(orphanedResourceNames))
			for _, n := range orphanedResourceNames {
				orphanedResourceNamesStr = append(orphanedResourceNamesStr, n.String())
			}
			mod.logger.Warnw("Some resources failed to re-add after crashed module restart and will be rebuilt",
				"module", mod.cfg.Name,
				"resources_to_be_rebuilt", orphanedResourceNamesStr)
			unlock()
			mgr.handleOrphanedResources(mgr.restartCtx, orphanedResourceNames)
		}

		mod.logger.Infow("Module resources successfully re-added after module restart",
			"module", mod.cfg.Name,
			"resources", restoredResourceNamesStr)
		return
	}
}

// attemptRestart will attempt to restart the module process. It returns nil
// on success and an error in case of failure. In the failure case it ensures
// that the failed process is killed and will not be restarted by pexec or an
// OUE handler.
func (mgr *Manager) attemptRestart(ctx context.Context, mod *module) error {
	var success, processRestarted bool
	defer func() {
		if !success {
			if processRestarted {
				if err := mod.stopProcess(); err != nil {
					msg := "Error while stopping process of crashed module"
					mgr.logger.Errorw(msg, "module", mod.cfg.Name, "error", err)
				}
			}
			mod.cleanupAfterCrash(mgr)
		}
	}()

	mgr.logger.CInfow(ctx, "Attempting to restart crashed module", "module", mod.cfg.Name)

	// No need to check mgr.untrustedEnv, as we're restarting the same
	// executable we were given for initial module addition.

	cleanup := rutils.SlowLogger(
		ctx, "Waiting for module to complete restart and re-registration", "module", mod.cfg.Name, mod.logger)
	defer cleanup()

	// There is a potential race here where the process starts but then crashes,
	// causing its OUE handler to spawn another restart attempt even though we've
	// determined the startup to be a failure and want the new process to stay
	// dead. To prevent this we wrap the new OUE handler in another function and
	// block its execution until we have determined startup success or failure,
	// at which point it exits early w/o attempting a restart on failure or
	// continues with the normal OUE execution on success.
	blockRestart := make(chan struct{})
	defer close(blockRestart)
	oue := func(oueCtx context.Context, exitCode int) bool {
		<-blockRestart
		if !success {
			return false
		}
		return mgr.newOnUnexpectedExitHandler(ctx, mod)(oueCtx, exitCode)
	}

	if err := mgr.startModuleProcess(mod, oue); err != nil {
		mgr.logger.Errorw("Error while restarting crashed module",
			"module", mod.cfg.Name, "error", err)
		return err
	}
	processRestarted = true

	if err := mod.dial(); err != nil {
		mgr.logger.CErrorw(ctx, "Error while dialing restarted module",
			"module", mod.cfg.Name, "error", err)
		return err
	}

	if err := mod.checkReady(ctx, mgr.parentAddr(mod)); err != nil {
		mgr.logger.CErrorw(ctx, "Error while waiting for restarted module to be ready",
			"module", mod.cfg.Name, "error", err)
		return err
	}

	if pc := mod.sharedConn.PeerConn(); mgr.modPeerConnTracker != nil && pc != nil {
		mgr.modPeerConnTracker.Add(mod.cfg.Name, pc)
	}
	mod.registerResourceModels(mgr)
	success = true
	return nil
}

// FirstRun is runs a module-specific setup script.
func (mgr *Manager) FirstRun(ctx context.Context, conf config.Module) error {
	pkgsDir := packages.LocalPackagesDir(mgr.packagesDir)

	// This value is normally set on a field on the [module] struct but it seems like we can safely get it on demand.
	var dataDir string
	if mgr.moduleDataParentDir != "" {
		var err error
		// TODO: why isn't conf.Name being sanitized like PackageConfig.SanitizedName?
		dataDir, err = rutils.SafeJoinDir(mgr.moduleDataParentDir, conf.Name)
		if err != nil {
			return err
		}
	}
	env := getFullEnvironment(conf, pkgsDir, dataDir, mgr.viamHomeDir)

	return conf.FirstRun(ctx, pkgsDir, dataDir, env, mgr.logger)
}

func getFullEnvironment(
	cfg config.Module,
	packagesDir string,
	dataDir string,
	viamHomeDir string,
) map[string]string {
	environment := map[string]string{
		"VIAM_HOME":        viamHomeDir,
		"VIAM_MODULE_DATA": dataDir,
		"VIAM_MODULE_NAME": cfg.Name,
	}
	if cfg.Type == config.ModuleTypeRegistry {
		environment["VIAM_MODULE_ID"] = cfg.ModuleID
	}

	// For local modules, we set VIAM_MODULE_ROOT to the parent directory of the unpacked module.
	// VIAM_MODULE_ROOT is filled out by app.viam.com in cloud robots.
	if cfg.Type == config.ModuleTypeLocal {
		moduleRoot, err := cfg.ExeDir(packages.LocalPackagesDir(packagesDir))
		// err should never not be nil since we are working with local modules
		if err == nil {
			environment["VIAM_MODULE_ROOT"] = moduleRoot
		}
	}

	// Overwrite the base environment variables with the module's environment variables (if specified)
	for key, value := range cfg.Environment {
		environment[key] = value
	}
	return environment
}

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps resource.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, resource.RemoveRemoteName(dep).String())
	}
	return depStrings
}

// getModuleDataParentDirectory generates the Manager's moduleDataParentDirectory.
// This directory should contain exactly one directory for each module present on the modmanager
// For cloud robots, it will generate a directory in the form:
// options.ViamHomeDir/module-data/<cloud-robot-id>
// For local robots, it should be in the form
// options.ViamHomeDir/module-data/local.
// For local robots in a testing environment (where no cloud ID is set), it should be in the form:
// [temp-dir]/module-data/local-testing-[random-string-to-avoid-collisions].
//
// If no ViamHomeDir is provided, this will return an empty moduleDataParentDirectory (and no module data directories will be created).
func getModuleDataParentDirectory(options modmanageroptions.Options) string {
	// if the home directory is empty, this is probably being run from an unrelated test
	// and creating a file could lead to race conditions
	if options.ViamHomeDir == "" {
		return ""
	}
	robotID := options.RobotCloudID
	if robotID == "" {
		if testing.Testing() {
			return filepath.Join(os.TempDir(), parentModuleDataFolderName, "local-testing-"+utils.RandomAlphaString(5))
		}

		robotID = "local"
	}
	return filepath.Join(options.ViamHomeDir, parentModuleDataFolderName, robotID)
}
