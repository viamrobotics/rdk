// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	robotpb "go.viam.com/api/robot/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/ftdc"
	"go.viam.com/rdk/ftdc/sys"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/packages"
	rutils "go.viam.com/rdk/utils"
)

// tcpPortRange is the beginning of the port range. Only used when ViamTCPSockets() = true.
const tcpPortRange = 13500

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
	ctx context.Context, parentAddr string, logger logging.Logger, options modmanageroptions.Options,
) modmaninterface.ModuleManager {
	restartCtx, restartCtxCancel := context.WithCancel(ctx)
	ret := &Manager{
		logger:                  logger.Sublogger("modmanager"),
		modules:                 moduleMap{},
		parentAddr:              parentAddr,
		rMap:                    resourceModuleMap{},
		untrustedEnv:            options.UntrustedEnv,
		viamHomeDir:             options.ViamHomeDir,
		moduleDataParentDir:     getModuleDataParentDirectory(options),
		removeOrphanedResources: options.RemoveOrphanedResources,
		restartCtx:              restartCtx,
		restartCtxCancel:        restartCtxCancel,
		packagesDir:             options.PackagesDir,
		ftdc:                    options.FTDC,
	}
	ret.nextPort.Store(tcpPortRange)
	return ret
}

type module struct {
	cfg        config.Module
	dataDir    string
	process    pexec.ManagedProcess
	handles    modlib.HandlerMap
	sharedConn rdkgrpc.SharedConn
	client     pb.ModuleServiceClient
	// robotClient supplements the ModuleServiceClient client to serve select robot level methods from the module server
	// such as the DiscoverComponents API
	robotClient robotpb.RobotServiceClient
	addr        string
	resources   map[resource.Name]*addedResource
	// resourcesMu must be held if the `resources` field is accessed without
	// write-locking the module manager.
	resourcesMu sync.Mutex

	// pendingRemoval allows delaying module close until after resources within it are closed
	pendingRemoval bool

	// inStartup stores whether or not the manager of the OnUnexpectedExit function
	// is trying to start up this module; inRecoveryLock guards the execution of an
	// OnUnexpectedExit function for this module.
	//
	// NOTE(benjirewis): Using just an atomic boolean is not sufficient, as OUE
	// functions for the same module cannot overlap and should not continue after
	// another OUE has finished.
	inStartup      atomic.Bool
	inRecoveryLock sync.Mutex
	logger         logging.Logger
	ftdc           *ftdc.FTDC
	// port stores the listen port of this module when ViamTCPSockets() = true.
	port int
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
	parentAddr   string
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
	removeOrphanedResources func(ctx context.Context, rNames []resource.Name)
	restartCtx              context.Context
	restartCtxCancel        context.CancelFunc
	ftdc                    *ftdc.FTDC
	// nextPort manages ports when ViamTCPSockets() = true.
	nextPort atomic.Int32
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

// An allowed list of specific viam namespace modules. We want to allow running some of our official
// modules even in an untrusted environment.
var allowedModules = map[string]bool{
	"viam:raspberry-pi": true,
}

// Checks if the modules added in an untrusted environment are Viam modules
// and returns `true` and a list of their configs if any exist in the passed-in slice.
func checkIfAllowed(confs ...config.Module) (
	allowed bool /*false*/, newConfs []config.Module,
) {
	for _, conf := range confs {
		if ok := allowedModules[conf.ModuleID]; ok {
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

		// this is done in config validation but partial start rules require us to check again
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
		port:      int(mgr.nextPort.Add(1)),
	}

	if err := mgr.startModule(ctx, mod); err != nil {
		return err
	}
	return nil
}

func (mgr *Manager) startModuleProcess(mod *module) error {
	return mod.startProcess(
		mgr.restartCtx,
		mgr.parentAddr,
		mgr.newOnUnexpectedExitHandler(mod),
		mgr.viamHomeDir,
		mgr.packagesDir,
	)
}

func (mgr *Manager) startModule(ctx context.Context, mod *module) error {
	// add calls startProcess, which can also be called by the OUE handler in the attemptRestart
	// call. Both of these involve owning a lock, so in unhappy cases of malformed modules
	// this can lead to a deadlock. To prevent this, we set inStartup here to indicate to
	// the OUE handler that it shouldn't act while add is still processing.
	mod.inStartup.Store(true)
	defer mod.inStartup.Store(false)

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

	cleanup := rutils.SlowStartupLogger(
		ctx, "Waiting for module to complete startup and registration", "module", mod.cfg.Name, mod.logger)
	defer cleanup()

	if err := mgr.startModuleProcess(mod); err != nil {
		return errors.WithMessage(err, "error while starting module "+mod.cfg.Name)
	}

	if err := mod.dial(); err != nil {
		return errors.WithMessage(err, "error while dialing module "+mod.cfg.Name)
	}

	if err := mod.checkReady(ctx, mgr.parentAddr); err != nil {
		return errors.WithMessage(err, "error while waiting for module to be ready "+mod.cfg.Name)
	}

	mod.registerResources(mgr)
	mgr.modules.Store(mod.cfg.Name, mod)
	mod.logger.Infow("Module successfully added", "module", mod.cfg.Name)
	success = true
	return nil
}

// Reconfigure reconfigures an existing resource module and returns the names of
// now orphaned resources.
func (mgr *Manager) Reconfigure(ctx context.Context, conf config.Module) ([]resource.Name, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, exists := mgr.modules.Load(conf.Name)
	if !exists {
		return nil, errors.Errorf("cannot reconfigure module %s as it does not exist", conf.Name)
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

	// If module handles no resources, remove it now. Otherwise mark it
	// pendingRemoval for eventual removal after last handled resource has been
	// closed.
	if len(handledResources) == 0 {
		return nil, mgr.closeModule(mod, false)
	}

	var orphanedResourceNames []resource.Name
	var orphanedResourceNameStrings []string
	for name := range handledResources {
		orphanedResourceNames = append(orphanedResourceNames, name)
		orphanedResourceNameStrings = append(orphanedResourceNameStrings, name.String())
	}
	mgr.logger.Infow("Resources handled by removed module will be removed",
		"module", mod.cfg.Name, "resources", orphanedResourceNameStrings)
	mod.pendingRemoval = true
	return orphanedResourceNames, nil
}

// closeModule attempts to cleanly shut down the module process. It does not wait on module recovery processes,
// as they are running outside code and may have unexpected behavior.
func (mgr *Manager) closeModule(mod *module, reconfigure bool) error {
	// resource manager should've removed these cleanly if this isn't a reconfigure
	if !reconfigure && len(mod.resources) != 0 {
		mod.logger.Warnw("Forcing removal of module with active resources", "module", mod.cfg.Name)
	}

	// need to actually close the resources within the module itself before stopping
	for res := range mod.resources {
		_, err := mod.client.RemoveResource(context.Background(), &pb.RemoveResourceRequest{Name: res.String()})
		if err != nil {
			mod.logger.Errorw("Error removing resource", "module", mod.cfg.Name, "resource", res.Name, "error", err)
		} else {
			mod.logger.Infow("Successfully removed resource from module", "module", mod.cfg.Name, "resource", res.Name)
		}
	}

	if err := mod.stopProcess(); err != nil {
		return errors.WithMessage(err, "error while stopping module "+mod.cfg.Name)
	}

	if err := mod.sharedConn.Close(); err != nil {
		mod.logger.Warnw("Error closing connection to module", "error", err)
	}

	mod.deregisterResources()

	mgr.rMap.Range(func(r resource.Name, m *module) bool {
		if m == mod {
			mgr.rMap.Delete(r)
		}
		return true
	})
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

func (mgr *Manager) addResourceWithWriteLock(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
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
	if err != nil {
		return err
	}

	// if the module is marked for removal, actually remove it when the final resource is closed
	if mod.pendingRemoval && len(mod.resources) == 0 {
		err = multierr.Combine(err, mgr.closeModule(mod, false))
	}
	return err
}

// ValidateConfig determines whether the given config is valid and returns its implicit
// dependencies.
func (mgr *Manager) ValidateConfig(ctx context.Context, conf resource.Config) ([]string, error) {
	mod, ok := mgr.getModule(conf)
	if !ok {
		return nil,
			errors.Errorf("no module registered to serve resource api %s and model %s",
				conf.API, conf.Model)
	}

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	// Override context with new timeout.
	var cancel func()
	ctx, cancel = context.WithTimeout(ctx, validateConfigTimeout)
	defer cancel()

	resp, err := mod.client.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: confProto})
	// Swallow "Unimplemented" gRPC errors from modules that lack ValidateConfig
	// receiving logic.
	if err != nil && status.Code(err) == codes.Unimplemented {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return resp.Dependencies, nil
}

// ResolveImplicitDependenciesInConfig mutates the passed in diff to add modular implicit dependencies to added
// and modified resources. It also puts modular resources whose module has been modified or added in conf.Added if
// they are not already there since the resources themselves are not necessarily new.
func (mgr *Manager) ResolveImplicitDependenciesInConfig(ctx context.Context, conf *config.Diff) error {
	// NOTE(benji): We could simplify some of the following `continue`
	// conditional clauses to a single clause, but we split them for readability.
	for _, c := range conf.Right.Components {
		mod, ok := mgr.getModule(c)
		if !ok {
			// continue if this component is not being provided by a module.
			continue
		}

		lenModified, lenAdded := len(conf.Modified.Modules), len(conf.Added.Modules)
		deltaModules := make([]config.Module, lenModified, lenModified+lenAdded)
		copy(deltaModules, conf.Modified.Modules)
		deltaModules = append(deltaModules, conf.Added.Modules...)

		if !slices.ContainsFunc(deltaModules, func(elem config.Module) bool {
			return elem.Name == mod.cfg.Name
		}) {
			// continue if this modular component is not being handled by a modified
			// or added module.
			continue
		}
		if slices.ContainsFunc(conf.Added.Components, func(elem resource.Config) bool {
			return elem.Name == c.Name
		}) {
			// continue if this modular component handled by a modified module is
			// already in conf.Added.Components.
			continue
		}

		// Add modular component to conf.Added.Components.
		conf.Added.Components = append(conf.Added.Components, c)
		// If component is in conf.Modified, the user modified a module and its
		// component at the same time. Remove that resource from conf.Modified so
		// the restarted module receives an AddResourceRequest and not a
		// ReconfigureResourceRequest.
		conf.Modified.Components = slices.DeleteFunc(
			conf.Modified.Components, func(elem resource.Config) bool { return elem.Name == c.Name })
	}
	for _, s := range conf.Right.Services {
		mod, ok := mgr.getModule(s)
		if !ok {
			// continue if this service is not being provided by a module.
			continue
		}
		if !slices.ContainsFunc(conf.Modified.Modules, func(elem config.Module) bool {
			return elem.Name == mod.cfg.Name
		}) {
			// continue if this modular service is not being handled by a modified
			// module.
			continue
		}
		if slices.ContainsFunc(conf.Added.Services, func(elem resource.Config) bool {
			return elem.Name == s.Name
		}) {
			// continue if this modular service handled by a modified module is
			// already in conf.Added.Services.
			continue
		}

		// Add modular service to conf.Added.Services.
		conf.Added.Services = append(conf.Added.Services, s)
		//  If service is in conf.Modified, the user modified a module and its
		//  service at the same time. Remove that resource from conf.Modified so
		//  the restarted module receives an AddResourceRequest and not a
		//  ReconfigureResourceRequest.
		conf.Modified.Services = slices.DeleteFunc(
			conf.Modified.Services, func(elem resource.Config) bool { return elem.Name == s.Name })
	}

	// If something was added or modified, go through components and services in
	// diff.Added and diff.Modified, call Validate on all those that are modularized,
	// and store implicit dependencies.
	validateModularResources := func(confs []resource.Config) {
		for i, c := range confs {
			if mgr.Provides(c) {
				implicitDeps, err := mgr.ValidateConfig(ctx, c)
				if err != nil {
					mgr.logger.CErrorw(ctx, "Modular config validation error found in resource: "+c.Name, "error", err)
					continue
				}

				// Modify resource config to add its implicit dependencies.
				confs[i].ImplicitDependsOn = implicitDeps
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
func (mgr *Manager) newOnUnexpectedExitHandler(mod *module) func(exitCode int) bool {
	return func(exitCode int) bool {
		mod.inRecoveryLock.Lock()
		defer mod.inRecoveryLock.Unlock()
		if mod.inStartup.Load() {
			return false
		}

		mod.inStartup.Store(true)
		defer mod.inStartup.Store(false)

		// Log error immediately, as this is unexpected behavior.
		mod.logger.Errorw(
			"Module has unexpectedly exited.", "module", mod.cfg.Name, "exit_code", exitCode,
		)

		if err := mod.sharedConn.Close(); err != nil {
			mod.logger.Warnw("Error closing connection to crashed module. Continuing restart attempt",
				"error", err)
		}

		if mgr.ftdc != nil {
			mgr.ftdc.Remove(mod.getFTDCName())
		}

		// If attemptRestart returns any orphaned resource names, restart failed,
		// and we should remove orphaned resources. Since we handle process
		// restarting ourselves, return false here so goutils knows not to attempt
		// a process restart.
		if orphanedResourceNames := mgr.attemptRestart(mgr.restartCtx, mod); orphanedResourceNames != nil {
			if mgr.removeOrphanedResources != nil {
				mgr.removeOrphanedResources(mgr.restartCtx, orphanedResourceNames)
				mod.logger.Debugw(
					"Removed resources after failed module restart",
					"module", mod.cfg.Name,
					"resources", resource.NamesToStrings(orphanedResourceNames),
				)
			}
			return false
		}
		mod.logger.Infow("Module successfully restarted, re-adding resources", "module", mod.cfg.Name)

		// Otherwise, add old module process' resources to new module; warn if new
		// module cannot handle old resource and remove it from mod.resources.
		// Finally, handle orphaned resources.
		var orphanedResourceNames []resource.Name
		for name, res := range mod.resources {
			// The `addResource` method might still be executing for this resource with a
			// read lock, so we execute it here with a write lock to make sure it doesn't
			// run concurrently.
			if _, err := mgr.addResourceWithWriteLock(mgr.restartCtx, res.conf, res.deps); err != nil {
				mod.logger.Warnw("Error while re-adding resource to module",
					"resource", name, "module", mod.cfg.Name, "error", err)
				mgr.rMap.Delete(name)

				mod.resourcesMu.Lock()
				delete(mod.resources, name)
				mod.resourcesMu.Unlock()

				orphanedResourceNames = append(orphanedResourceNames, name)
			}
		}
		if len(orphanedResourceNames) > 0 && mgr.removeOrphanedResources != nil {
			mgr.removeOrphanedResources(mgr.restartCtx, orphanedResourceNames)
		}

		mod.logger.Infow("Module resources successfully re-added after module restart", "module", mod.cfg.Name)
		return false
	}
}

// attemptRestart will attempt to restart the module up to three times and
// return the names of now orphaned resources.
func (mgr *Manager) attemptRestart(ctx context.Context, mod *module) []resource.Name {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// deregister crashed module's resources, and let later checkReady reset m.handles
	// before reregistering.
	mod.deregisterResources()

	var orphanedResourceNames []resource.Name
	for name := range mod.resources {
		orphanedResourceNames = append(orphanedResourceNames, name)
	}

	// Attempt to remove module's .sock file if module did not remove it
	// already.
	rutils.RemoveFileNoError(mod.addr)

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

	if ctx.Err() != nil {
		mgr.logger.CInfow(
			ctx, "Will not attempt to restart crashed module", "module", mod.cfg.Name, "reason", ctx.Err().Error(),
		)
		return orphanedResourceNames
	}
	mgr.logger.CInfow(ctx, "Attempting to restart crashed module", "module", mod.cfg.Name)

	// No need to check mgr.untrustedEnv, as we're restarting the same
	// executable we were given for initial module addition.

	cleanup := rutils.SlowStartupLogger(
		ctx, "Waiting for module to complete restart and re-registration", "module", mod.cfg.Name, mod.logger)
	defer cleanup()

	// Attempt to restart module process 3 times.
	for attempt := 1; attempt < 4; attempt++ {
		if err := mgr.startModuleProcess(mod); err != nil {
			mgr.logger.Errorw("Error while restarting crashed module", "restart attempt",
				attempt, "module", mod.cfg.Name, "error", err)
			if attempt == 3 {
				// return early upon last attempt failure.
				return orphanedResourceNames
			}
		} else {
			break
		}

		// Wait with a bit of backoff. Exit early if context has errorred.
		if !utils.SelectContextOrWait(ctx, time.Duration(attempt)*oueRestartInterval) {
			mgr.logger.CInfow(
				ctx, "Will not continue to attempt restarting crashed module", "module", mod.cfg.Name, "reason", ctx.Err().Error(),
			)
			return orphanedResourceNames
		}
	}
	processRestarted = true

	if err := mod.dial(); err != nil {
		mgr.logger.CErrorw(ctx, "Error while dialing restarted module",
			"module", mod.cfg.Name, "error", err)
		return orphanedResourceNames
	}

	if err := mod.checkReady(ctx, mgr.parentAddr); err != nil {
		mgr.logger.CErrorw(ctx, "Error while waiting for restarted module to be ready",
			"module", mod.cfg.Name, "error", err)
		return orphanedResourceNames
	}

	mod.registerResources(mgr)

	success = true
	return nil
}

// dial will Dial the module and replace the underlying connection (if it exists) in m.conn.
func (m *module) dial() error {
	// TODO(PRODUCT-343): session support probably means interceptors here
	var err error
	addrToDial := m.addr
	if !rutils.TCPRegex.MatchString(addrToDial) {
		addrToDial = "unix://" + m.addr
	}
	conn, err := grpc.Dial( //nolint:staticcheck
		addrToDial,
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(rpc.MaxMessageSize)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			rdkgrpc.EnsureTimeoutUnaryClientInterceptor,
			grpc_retry.UnaryClientInterceptor(),
			operation.UnaryClientInterceptor,
		),
		grpc.WithChainStreamInterceptor(
			grpc_retry.StreamClientInterceptor(),
			operation.StreamClientInterceptor,
		),
	)
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	// Take the grpc over unix socket connection and add it to this `module`s `SharedConn`
	// object. This `m.sharedConn` object is referenced by all resources/components. `Client`
	// objects communicating with the module. If we're re-dialing after a restart, there may be
	// existing resource `Client`s objects. Rather than recreating clients with new information, we
	// choose to "swap out" the underlying connection object for those existing `Client`s.
	//
	// Resetting the `SharedConn` will also create a new WebRTC PeerConnection object. `dial`ing to
	// a module is followed by doing a `ReadyRequest` `ReadyResponse` exchange. If that exchange
	// contains a working WebRTC offer and answer, the PeerConnection will succeed in connecting. If
	// there is an error exchanging offers and answers, the PeerConnection object will be nil'ed
	// out.
	m.sharedConn.ResetConn(rpc.GrpcOverHTTPClientConn{ClientConn: conn}, m.logger)
	m.client = pb.NewModuleServiceClient(m.sharedConn.GrpcConn())
	m.robotClient = robotpb.NewRobotServiceClient(m.sharedConn.GrpcConn())
	return nil
}

// checkReady sends a `ReadyRequest` and waits for either a `ReadyResponse`, or a context
// cancelation.
func (m *module) checkReady(ctx context.Context, parentAddr string) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer cancelFunc()

	m.logger.CInfow(ctx, "Waiting for module to respond to ready request", "module", m.cfg.Name)

	req := &pb.ReadyRequest{ParentAddress: parentAddr}

	// Wait for gathering to complete. Pass the entire SDP as an offer to the `ReadyRequest`.
	var err error
	req.WebrtcOffer, err = m.sharedConn.GenerateEncodedOffer()
	if err != nil {
		m.logger.CWarnw(ctx, "Unable to generate offer for module PeerConnection. Ignoring.", "err", err)
	}

	for {
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		resp, err := m.client.Ready(ctxTimeout, req, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if !resp.Ready {
			// Module's can express that they are in a state:
			// - That is Capable of receiving and responding to gRPC commands
			// - But is "not ready" to take full responsibility of being a module
			//
			// Our behavior is to busy-poll until a module declares it is ready. But we otherwise do
			// not adjust timeouts based on this information.
			continue
		}

		err = m.sharedConn.ProcessEncodedAnswer(resp.WebrtcAnswer)
		if err != nil {
			m.logger.CWarnw(ctx, "Unable to create PeerConnection with module. Ignoring.", "err", err)
		}

		// The `ReadyRespones` also includes the Viam `API`s and `Model`s the module provides. This
		// will be used to construct "generic Client" objects that can execute gRPC commands for
		// methods that are not part of the viam-server's API proto.
		m.handles, err = modlib.NewHandlerMapFromProto(ctx, resp.Handlermap, m.sharedConn.GrpcConn())
		return err
	}
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
	env := getFullEnvironment(conf, dataDir, mgr.viamHomeDir)

	return conf.FirstRun(ctx, pkgsDir, dataDir, env, mgr.logger)
}

func (m *module) startProcess(
	ctx context.Context,
	parentAddr string,
	oue func(int) bool,
	viamHomeDir string,
	packagesDir string,
) error {
	var err error

	if rutils.ViamTCPSockets() {
		m.addr = "127.0.0.1:" + strconv.Itoa(m.port)
	} else {
		// append a random alpha string to the module name while creating a socket address to avoid conflicts
		// with old versions of the module.
		if m.addr, err = modlib.CreateSocketAddress(
			filepath.Dir(parentAddr), fmt.Sprintf("%s-%s", m.cfg.Name, utils.RandomAlphaString(5))); err != nil {
			return err
		}
	}

	// We evaluate the Module's ExePath absolutely in the viam-server process so that
	// setting the CWD does not cause issues with relative process names
	absoluteExePath, err := m.cfg.EvaluateExePath(packages.LocalPackagesDir(packagesDir))
	if err != nil {
		return err
	}
	moduleEnvironment := m.getFullEnvironment(viamHomeDir)
	// Prefer VIAM_MODULE_ROOT as the current working directory if present but fallback to the directory of the exepath
	moduleWorkingDirectory, ok := moduleEnvironment["VIAM_MODULE_ROOT"]
	if !ok {
		moduleWorkingDirectory = filepath.Dir(absoluteExePath)
		m.logger.CDebugw(ctx, "VIAM_MODULE_ROOT was not passed to module. Defaulting to module's working directory",
			"module", m.cfg.Name, "dir", moduleWorkingDirectory)
	} else {
		m.logger.CInfow(ctx, "Starting module in working directory", "module", m.cfg.Name, "dir", moduleWorkingDirectory)
	}

	pconf := pexec.ProcessConfig{
		ID:               m.cfg.Name,
		Name:             absoluteExePath,
		Args:             []string{m.addr},
		CWD:              moduleWorkingDirectory,
		Environment:      moduleEnvironment,
		Log:              true,
		OnUnexpectedExit: oue,
	}
	// Start module process with supplied log level or "debug" if none is
	// supplied and module manager has a DebugLevel logger.
	if m.cfg.LogLevel != "" {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, m.cfg.LogLevel))
	} else if m.logger.Level().Enabled(zapcore.DebugLevel) {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, "debug"))
	}

	m.process = pexec.NewManagedProcess(pconf, m.logger)

	if err := m.process.Start(context.Background()); err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	// Turn on process cpu/memory diagnostics for the module process. If there's an error, we
	// continue normally, just without FTDC.
	m.registerProcessWithFTDC()

	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()

	m.logger.CInfow(ctx, "Starting up module", "module", m.cfg.Name)
	rutils.LogViamEnvVariables("Starting module with following Viam environment variables", moduleEnvironment, m.logger)

	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(m.logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
				return rutils.NewModuleStartUpTimeoutError(m.cfg.Name)
			}
			return ctxTimeout.Err()
		case <-checkTicker.C:
			if errors.Is(m.process.Status(), os.ErrProcessDone) {
				return fmt.Errorf(
					"module %s exited too quickly after attempted startup; it might have a fatal runtime issue",
					m.cfg.Name,
				)
			}
		}
		if !rutils.TCPRegex.MatchString(m.addr) {
			// note: we don't do this check in TCP mode because TCP addresses are not file paths and will fail check.
			err = modlib.CheckSocketOwner(m.addr)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return errors.WithMessage(err, "module startup failed")
			}
		}
		break
	}
	return nil
}

func (m *module) stopProcess() error {
	if m.process == nil {
		return nil
	}

	m.logger.Infof("Stopping module: %s process", m.cfg.Name)

	// Attempt to remove module's .sock file if module did not remove it
	// already.
	defer func() {
		rutils.RemoveFileNoError(m.addr)

		// The system metrics "statser" is resilient to the process dying under the hood. An empty set
		// of metrics will be reported. Therefore it is safe to continue monitoring the module process
		// while it's in shutdown.
		if m.ftdc != nil {
			m.ftdc.Remove(m.getFTDCName())
		}
	}()

	// TODO(RSDK-2551): stop ignoring exit status 143 once Python modules handle
	// SIGTERM correctly.
	// Also ignore if error is that the process no longer exists.
	if err := m.process.Stop(); err != nil {
		if strings.Contains(err.Error(), errMessageExitStatus143) || strings.Contains(err.Error(), "no such process") {
			return nil
		}
		return err
	}

	return nil
}

func (m *module) killProcessGroup() {
	if m.process == nil {
		return
	}
	m.logger.Infof("Killing module: %s process", m.cfg.Name)
	m.process.KillGroup()
}

func (m *module) registerResources(mgr modmaninterface.ModuleManager) {
	for api, models := range m.handles {
		if _, ok := resource.LookupGenericAPIRegistration(api.API); !ok {
			resource.RegisterAPI(
				api.API,
				resource.APIRegistration[resource.Resource]{ReflectRPCServiceDesc: api.Desc},
			)
		}

		switch {
		case api.API.IsComponent():
			for _, model := range models {
				m.logger.Infow("Registering component API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
				// We must copy because the Discover closure func relies on api and model, but they are iterators and mutate.
				// Copying prevents mutation.
				modelCopy := model
				apiCopy := api
				resource.RegisterComponent(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger logging.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
					Discover: func(ctx context.Context, logger logging.Logger, extra map[string]interface{}) (interface{}, error) {
						extraStructPb, err := structpb.NewStruct(extra)
						if err != nil {
							return nil, err
						}

						//nolint:deprecated,staticcheck
						req := &robotpb.DiscoverComponentsRequest{
							Queries: []*robotpb.DiscoveryQuery{
								{Subtype: apiCopy.API.String(), Model: modelCopy.String(), Extra: extraStructPb},
							},
						}

						//nolint:deprecated,staticcheck
						res, err := m.robotClient.DiscoverComponents(ctx, req)
						if err != nil {
							m.logger.Errorf("error in modular DiscoverComponents: %s", err)
							return nil, err
						}
						switch len(res.Discovery) {
						case 0:
							return nil, errors.New("modular DiscoverComponents response did not contain any discoveries")
						case 1:
							return res.Discovery[0].Results.AsMap(), nil
						default:
							return nil, errors.New("modular DiscoverComponents response contains more than one discovery")
						}
					},
				})
			}
		case api.API.IsService():
			for _, model := range models {
				m.logger.Infow("Registering service API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
				resource.RegisterService(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger logging.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
				})
			}
		default:
			m.logger.Errorw("Invalid module type", "API type", api.API.Type)
		}
	}
}

func (m *module) deregisterResources() {
	for api, models := range m.handles {
		for _, model := range models {
			resource.Deregister(api.API, model)
		}
	}
	m.handles = nil
}

func (m *module) cleanupAfterStartupFailure() {
	if err := m.stopProcess(); err != nil {
		msg := "Error while stopping process of module that failed to start"
		m.logger.Errorw(msg, "module", m.cfg.Name, "error", err)
	}
	utils.UncheckedError(m.sharedConn.Close())
}

func (m *module) cleanupAfterCrash(mgr *Manager) {
	utils.UncheckedError(m.sharedConn.Close())
	mgr.rMap.Range(func(r resource.Name, mod *module) bool {
		if mod == m {
			mgr.rMap.Delete(r)
		}
		return true
	})
	mgr.modules.Delete(m.cfg.Name)
}

func (m *module) getFullEnvironment(viamHomeDir string) map[string]string {
	return getFullEnvironment(m.cfg, m.dataDir, viamHomeDir)
}

func (m *module) getFTDCName() string {
	return fmt.Sprintf("proc.modules.%s", m.process.ID())
}

func (m *module) registerProcessWithFTDC() {
	if m.ftdc == nil {
		return
	}

	pid, err := m.process.UnixPid()
	if err != nil {
		m.logger.Warnw("Module process has no pid. Cannot start ftdc.", "err", err)
		return
	}

	statser, err := sys.NewPidSysUsageStatser(pid)
	if err != nil {
		m.logger.Warnw("Cannot find /proc files", "err", err)
		return
	}

	m.ftdc.Add(m.getFTDCName(), statser)
}

func getFullEnvironment(
	cfg config.Module,
	dataDir string,
	viamHomeDir string,
) map[string]string {
	environment := map[string]string{
		"VIAM_HOME":        viamHomeDir,
		"VIAM_MODULE_DATA": dataDir,
	}
	if cfg.Type == config.ModuleTypeRegistry {
		environment["VIAM_MODULE_ID"] = cfg.ModuleID
	}
	// Overwrite the base environment variables with the module's environment variables (if specified)
	// VIAM_MODULE_ROOT is filled out by app.viam.com in cloud robots.
	for key, value := range cfg.Environment {
		environment[key] = value
	}
	return environment
}

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps resource.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}

// getModuleDataParentDirectory generates the Manager's moduleDataParentDirectory.
// This directory should contain exactly one directory for each module present on the modmanager
// For cloud robots, it will generate a directory in the form:
// options.ViamHomeDir/module-data/<cloud-robot-id>
// For local robots, it should be in the form
// options.ViamHomeDir/module-data/local.
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
		robotID = "local"
	}
	return filepath.Join(options.ViamHomeDir, parentModuleDataFolderName, robotID)
}
