// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pion/webrtc/v3"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	modlib "go.viam.com/rdk/module"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/module/modmaninterface"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
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
	ctx context.Context, parentAddr string, logger logging.Logger, options modmanageroptions.Options,
) modmaninterface.ModuleManager {
	restartCtx, restartCtxCancel := context.WithCancel(ctx)
	return &Manager{
		logger:                  logger,
		modules:                 moduleMap{},
		parentAddr:              parentAddr,
		rMap:                    resourceModuleMap{},
		untrustedEnv:            options.UntrustedEnv,
		viamHomeDir:             options.ViamHomeDir,
		moduleDataParentDir:     getModuleDataParentDirectory(options),
		removeOrphanedResources: options.RemoveOrphanedResources,
		restartCtx:              restartCtx,
		restartCtxCancel:        restartCtxCancel,
	}
}

type module struct {
	cfg       config.Module
	dataDir   string
	process   pexec.ManagedProcess
	handles   modlib.HandlerMap
	conn      rdkgrpc.ReconfigurableClientConn
	client    pb.ModuleServiceClient
	addr      string
	resources map[resource.Name]*addedResource

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

	peerConn   *webrtc.PeerConnection
	sharedConn *rdkgrpc.SharedConn
	pcReady    <-chan struct{}
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
	mu           sync.RWMutex
	logger       logging.Logger
	modules      moduleMap
	parentAddr   string
	rMap         resourceModuleMap
	untrustedEnv bool
	// viamHomeDir is the absolute path to the viam home directory. Ex: /home/walle/.viam
	// `viamHomeDir` may only be the empty string in testing
	viamHomeDir string
	// moduleDataParentDir is the absolute path to the current robots module data directory.
	// Ex: /home/walle/.viam/module-data/<cloud-robot-id>
	// it is empty if the modmanageroptions.Options.viamHomeDir was empty
	moduleDataParentDir     string
	removeOrphanedResources func(ctx context.Context, rNames []resource.Name)
	restartCtx              context.Context
	restartCtxCancel        context.CancelFunc
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

// Handles returns all the models for each module registered.
func (mgr *Manager) Handles() map[string]modlib.HandlerMap {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	res := map[string]modlib.HandlerMap{}

	mgr.modules.Range(func(n string, m *module) bool {
		res[n] = m.handles
		return true
	})

	return res
}

// Add adds and starts a new resource modules for each given module configuration.
//
// Each module configuration should have a unique name - if duplicate names are detected,
// then only the first duplicate instance will be processed and the rest will be ignored.
func (mgr *Manager) Add(ctx context.Context, confs ...config.Module) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.untrustedEnv {
		return errModularResourcesDisabled
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

			mgr.logger.CInfow(ctx, "Now adding module", "module", conf.Name)
			err := mgr.add(ctx, conf)
			if err != nil {
				mgr.logger.CErrorw(ctx, "Error adding module", "module", conf.Name, "error", err)
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

func (mgr *Manager) add(ctx context.Context, conf config.Module) error {
	_, exists := mgr.modules.Load(conf.Name)
	if exists {
		mgr.logger.CWarnw(ctx, "Not adding module that already exists", "module", conf.Name)
		return nil
	}

	var moduleDataDir string
	// only set the module data directory if the parent dir is present (which it might not be during tests)
	if mgr.moduleDataParentDir != "" {
		moduleDataDir = filepath.Join(mgr.moduleDataParentDir, conf.Name)
		// safety check to prevent exiting the moduleDataDirectory in case conf.Name ends up including characters like ".."
		if !strings.HasPrefix(filepath.Clean(moduleDataDir), filepath.Clean(mgr.moduleDataParentDir)) {
			return errors.Errorf("module %q would have a data directory %q outside of the module data directory %q",
				conf.Name, moduleDataDir, mgr.moduleDataParentDir)
		}
	}

	mod := &module{
		cfg:       conf,
		dataDir:   moduleDataDir,
		resources: map[resource.Name]*addedResource{},
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		mgr.logger.Debugw("Unable to create optional peer connection for module. Ignoring.", "module", conf.Name, "err", err)
	} else {
		mod.peerConn = pc
		mod.pcReady, err = rpc.ConfigureForRenegotiation(mod.peerConn, mgr.logger.AsZap())
		if err != nil {
			mgr.logger.Debugw("Unable to create optional renegotiation channel for module. Ignoring.", "module", conf.Name, "err", err)
		}
	}

	if err = mgr.startModule(ctx, mod); err != nil {
		return err
	}
	return nil
}

func (mgr *Manager) startModuleProcess(mod *module) error {
	return mod.startProcess(
		mgr.restartCtx,
		mgr.parentAddr,
		mgr.newOnUnexpectedExitHandler(mod),
		mgr.logger,
		mgr.viamHomeDir,
	)
}

func (mgr *Manager) startSlowStartupLogTicker(ctx context.Context, msg, modName string) func() {
	slowTicker := time.NewTicker(2 * time.Second)
	firstTick := true

	ctxWithCancel, cancel := context.WithCancel(ctx)
	startTime := time.Now()
	go func() {
		for {
			select {
			case <-slowTicker.C:
				elapsed := time.Since(startTime).Seconds()
				mgr.logger.CWarnw(ctx, msg, "module", modName, "time elapsed", elapsed)
				if firstTick {
					slowTicker.Reset(3 * time.Second)
					firstTick = false
				} else {
					slowTicker.Reset(5 * time.Second)
				}
			case <-ctxWithCancel.Done():
				return
			}
		}
	}()
	return func() { slowTicker.Stop(); cancel() }
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
			mod.cleanupAfterStartupFailure(mgr.logger)
		}
	}()

	// create the module's data directory
	if mod.dataDir != "" {
		mgr.logger.Infof("Creating data directory %q for module %q", mod.dataDir, mod.cfg.Name)
		if err := os.MkdirAll(mod.dataDir, 0o750); err != nil {
			return errors.WithMessage(err, "error while creating data directory for module "+mod.cfg.Name)
		}
	}

	cleanup := mgr.startSlowStartupLogTicker(ctx, "Waiting for module to complete startup and registration", mod.cfg.Name)
	defer cleanup()

	if err := mgr.startModuleProcess(mod); err != nil {
		return errors.WithMessage(err, "error while starting module "+mod.cfg.Name)
	}

	if err := mod.dial(); err != nil {
		return errors.WithMessage(err, "error while dialing module "+mod.cfg.Name)
	}

	if err := mod.checkReady(ctx, mgr.parentAddr, mgr.logger); err != nil {
		return errors.WithMessage(err, "error while waiting for module to be ready "+mod.cfg.Name)
	}

	mod.registerResources(mgr, mgr.logger)
	mgr.modules.Store(mod.cfg.Name, mod)
	mgr.logger.Infow("Module successfully added", "module", mod.cfg.Name)
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

	mgr.logger.CInfow(ctx, "Module configuration changed. Stopping the existing module process", "module", conf.Name)

	if err := mgr.closeModule(mod, true); err != nil {
		// If removal fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	mod.cfg = conf
	mod.resources = map[resource.Name]*addedResource{}

	mgr.logger.CInfow(ctx, "Existing module process stopped. Starting new module process", "module", conf.Name)

	if err := mgr.startModule(ctx, mod); err != nil {
		// If re-addition fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	mgr.logger.CInfow(ctx, "New module process is running and responding to gRPC requests", "module",
		mod.cfg.Name, "module address", mod.addr)

	mgr.logger.CInfow(ctx, "Resources handled by reconfigured module will be re-added to new module process",
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

func (mgr *Manager) closeModule(mod *module, reconfigure bool) error {
	// resource manager should've removed these cleanly if this isn't a reconfigure
	if !reconfigure && len(mod.resources) != 0 {
		mgr.logger.Warnw("Forcing removal of module with active resources", "module", mod.cfg.Name)
	}

	// need to actually close the resources within the module itself before stopping
	for res := range mod.resources {
		_, err := mod.client.RemoveResource(context.Background(), &pb.RemoveResourceRequest{Name: res.String()})
		if err != nil {
			mgr.logger.Errorw("Error removing resource", "module", mod.cfg.Name, "resource", res.Name, "error", err)
		} else {
			mgr.logger.Infow("Successfully removed resource from module", "module", mod.cfg.Name, "resource", res.Name)
		}
	}

	if err := mod.stopProcess(); err != nil {
		return errors.WithMessage(err, "error while stopping module "+mod.cfg.Name)
	}

	if mod.sharedConn != nil {
		if err := mod.sharedConn.Close(); err != nil {
			return errors.WithMessage(err, "error while closing shared connection from module "+mod.cfg.Name)
		}
	} else {
		if mod.peerConn != nil {
			if err := mod.peerConn.Close(); err != nil {
				mgr.logger.Debugw("error closing optional webrtc peer connection", "err", err)
			}
		}

		if err := mod.conn.Close(); err != nil {
			return errors.WithMessage(err, "error while closing connection from module "+mod.cfg.Name)
		}
	}

	mod.deregisterResources()

	mgr.rMap.Range(func(r resource.Name, m *module) bool {
		if m == mod {
			mgr.rMap.Delete(r)
		}
		return true
	})
	mgr.modules.Delete(mod.cfg.Name)

	mgr.logger.Infow("Module successfully closed", "module", mod.cfg.Name)
	return nil
}

// AddResource tells a component module to configure a new component.
func (mgr *Manager) AddResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.addResource(ctx, conf, deps)
}

func (mgr *Manager) addResource(ctx context.Context, conf resource.Config, deps []string) (resource.Resource, error) {
	mod, ok := mgr.getModule(conf)
	if !ok {
		return nil, errors.Errorf("no active module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	mgr.logger.CInfow(ctx, "Adding resource to module", "resource", conf.Name, "module", mod.cfg.Name)

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	_, err = mod.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}
	mgr.rMap.Store(conf.ResourceName(), mod)
	mod.resources[conf.ResourceName()] = &addedResource{conf, deps}

	apiInfo, ok := resource.LookupGenericAPIRegistration(conf.API)
	if !ok || apiInfo.RPCClient == nil {
		mgr.logger.CWarnw(ctx, "No built-in grpc client for modular resource", "resource", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), mod.sharedConn), nil
	}

	return apiInfo.RPCClient(ctx, mod.sharedConn, "", conf.ResourceName(), mgr.logger)
}

// ReconfigureResource updates/reconfigures a modular component with a new configuration.
func (mgr *Manager) ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	mod, ok := mgr.getModule(conf)
	if !ok {
		return errors.Errorf("no module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	mgr.logger.CInfow(ctx, "Reconfiguring resource for module", "resource", conf.Name, "module", mod.cfg.Name)

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return err
	}
	_, err = mod.client.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return err
	}
	mod.resources[conf.ResourceName()] = &addedResource{conf, deps}

	return nil
}

// Configs returns a slice of config.Module representing the currently managed
// modules.
func (mgr *Manager) Configs() []config.Module {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	var configs []config.Module
	mgr.modules.Range(func(_ string, mod *module) bool {
		configs = append(configs, mod.cfg)
		return true
	})
	return configs
}

// Provides returns true if a component/service config WOULD be handled by a module.
func (mgr *Manager) Provides(conf resource.Config) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	_, ok := mgr.getModule(conf)
	return ok
}

// IsModularResource returns true if an existing resource IS handled by a module.
func (mgr *Manager) IsModularResource(name resource.Name) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
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

	mgr.logger.CInfow(ctx, "Removing resource for module", "resource", name.String(), "module", mod.cfg.Name)

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
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
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
// and modified resources. It also puts modular resources whose module has been modified in conf.Added if they
// are not already there.
func (mgr *Manager) ResolveImplicitDependenciesInConfig(ctx context.Context, conf *config.Diff) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	// NOTE(benji): We could simplify some of the following `continue`
	// conditional clauses to a single clause, but we split them for readability.
	for _, c := range conf.Right.Components {
		mod, ok := mgr.getModule(c)
		if !ok {
			// continue if this component is not being provided by a module.
			continue
		}
		if !slices.ContainsFunc(conf.Modified.Modules, func(elem config.Module) bool {
			return elem.Name == mod.cfg.Name
		}) {
			// continue if this modular component is not being handled by a modified
			// module.
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
		mgr.logger.Errorw(
			"Module has unexpectedly exited. Attempting to restart it",
			"module", mod.cfg.Name,
			"exit_code", exitCode,
		)

		// close client connection, we will re-dial as part of restart attempts.
		if mod.sharedConn != nil {
			if err := mod.sharedConn.Close(); err != nil {
				mgr.logger.Warnw("Error while closing shared connection to crashed module. Continuing restart attempt",
					"module", mod.cfg.Name, "error", err)
			}
		} else {
			if mod.peerConn != nil {
				if err := mod.peerConn.Close(); err != nil {
					mgr.logger.Debugw("error closing optional webrtc peer connection", "err", err)
				}
			}

			if err := mod.conn.Close(); err != nil {
				mgr.logger.Warnw("Error while closing connection to crashed module. Continuing restart attempt",
					"module", mod.cfg.Name, "error", err)
			}
		}

		// If attemptRestart returns any orphaned resource names, restart failed,
		// and we should remove orphaned resources. Since we handle process
		// restarting ourselves, return false here so goutils knows not to attempt
		// a process restart.
		if orphanedResourceNames := mgr.attemptRestart(mgr.restartCtx, mod); orphanedResourceNames != nil {
			if mgr.removeOrphanedResources != nil {
				mgr.removeOrphanedResources(mgr.restartCtx, orphanedResourceNames)
			}
			return false
		}
		mgr.logger.Infow("Module successfully restarted, re-adding resources", "module", mod.cfg.Name)

		// Otherwise, add old module process' resources to new module; warn if new
		// module cannot handle old resource and remove it from mod.resources.
		// Finally, handle orphaned resources.
		var orphanedResourceNames []resource.Name
		for name, res := range mod.resources {
			if _, err := mgr.AddResource(mgr.restartCtx, res.conf, res.deps); err != nil {
				mgr.logger.Warnw("Error while re-adding resource to module",
					"resource", name, "module", mod.cfg.Name, "error", err)
				mgr.rMap.Delete(name)
				delete(mod.resources, name)
				orphanedResourceNames = append(orphanedResourceNames, name)
			}
		}
		if mgr.removeOrphanedResources != nil {
			mgr.removeOrphanedResources(mgr.restartCtx, orphanedResourceNames)
		}

		mgr.logger.Infow("Module resources successfully re-added after module restart", "module", mod.cfg.Name)
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

	var success bool
	defer func() {
		if !success {
			mod.cleanupAfterCrash(mgr)
		}
	}()

	// No need to check mgr.untrustedEnv, as we're restarting the same
	// executable we were given for initial module addition.

	cleanup := mgr.startSlowStartupLogTicker(ctx, "Waiting for module to complete restart and re-registration", mod.cfg.Name)
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

		// Wait with a bit of backoff.
		utils.SelectContextOrWait(ctx, time.Duration(attempt)*oueRestartInterval)
	}

	if err := mod.dial(); err != nil {
		mgr.logger.CErrorw(ctx, "Error while dialing restarted module",
			"module", mod.cfg.Name, "error", err)
		return orphanedResourceNames
	}

	if err := mod.checkReady(ctx, mgr.parentAddr, mgr.logger); err != nil {
		mgr.logger.CErrorw(ctx, "Error while waiting for restarted module to be ready",
			"module", mod.cfg.Name, "error", err)
		return orphanedResourceNames
	}

	mod.registerResources(mgr, mgr.logger)

	success = true
	return nil
}

// dial will Dial the module and replace the underlying connection (if it exists) in m.conn.
func (m *module) dial() error {
	// TODO(PRODUCT-343): session support probably means interceptors here
	var err error
	conn, err := grpc.Dial(
		"unix://"+m.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
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
	m.conn.ReplaceConn(rpc.GrpcOverHTTPClientConn{ClientConn: conn})
	m.client = pb.NewModuleServiceClient(&m.conn)
	return nil
}

func (m *module) checkReady(ctx context.Context, parentAddr string, logger logging.Logger) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(logger))
	defer cancelFunc()

	logger.CInfow(ctx, "Waiting for module to respond to ready request", "module", m.cfg.Name)

	req := &pb.ReadyRequest{ParentAddress: parentAddr}

	// Wait for gathering to complete. Pass the entire SDP as an offer to the `ReadyRequest`.
	if sdp, err := generateSDP(m.peerConn); err == nil {
		req.WebrtcOffer = sdp
	} else {
		logger.Debugw("Unable to generate SDP for peerconn. Ignoring.", "err", err)
	}

	for {
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		resp, err := m.client.Ready(ctxTimeout, req, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if resp.Ready {
			webrtcConnectSucceeded := true
			if err = connect(m.peerConn, resp.WebrtcAnswer, logger); err != nil {
				logger.Debugw("Unable to create PeerConnection with module. Ignoring.", "err", err)
				webrtcConnectSucceeded = false
			}
			m.sharedConn = rdkgrpc.NewSharedConn(&m.conn, m.peerConn, logger)
			if webrtcConnectSucceeded {
				<-m.pcReady
			}
			m.handles, err = modlib.NewHandlerMapFromProto(ctx, resp.Handlermap, &m.conn)
			return err
		}
	}
}

func (m *module) startProcess(
	ctx context.Context,
	parentAddr string,
	oue func(int) bool,
	logger logging.Logger,
	viamHomeDir string,
) error {
	var err error
	// append a random alpha string to the module name while creating a socket address to avoid conflicts
	// with old versions of the module.
	if m.addr, err = modlib.CreateSocketAddress(
		filepath.Dir(parentAddr), fmt.Sprintf("%s-%s", m.cfg.Name, utils.RandomAlphaString(5))); err != nil {
		return err
	}

	// We evaluate the Module's ExePath absolutely in the viam-server process so that
	// setting the CWD does not cause issues with relative process names
	absoluteExePath, err := filepath.Abs(m.cfg.ExePath)
	if err != nil {
		return err
	}
	moduleEnvironment := m.getFullEnvironment(viamHomeDir)
	// Prefer VIAM_MODULE_ROOT as the current working directory if present but fallback to the directory of the exepath
	moduleWorkingDirectory, ok := moduleEnvironment["VIAM_MODULE_ROOT"]
	if !ok {
		moduleWorkingDirectory = filepath.Dir(absoluteExePath)
		logger.CWarnw(ctx, "VIAM_MODULE_ROOT was not passed to module. Defaulting to module's working directory",
			"module", m.cfg.Name, "dir", moduleWorkingDirectory)
	} else {
		logger.CInfow(ctx, "Starting module in working directory", "module", m.cfg.Name, "dir", moduleWorkingDirectory)
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
	} else if logger.Level().Enabled(zapcore.DebugLevel) {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, "debug"))
	}

	m.process = pexec.NewManagedProcess(pconf, logger.AsZap())

	if err := m.process.Start(context.Background()); err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	checkTicker := time.NewTicker(100 * time.Millisecond)
	defer checkTicker.Stop()

	logger.CInfow(ctx, "Starting up module", "module", m.cfg.Name)
	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetModuleStartupTimeout(logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			if errors.Is(ctxTimeout.Err(), context.DeadlineExceeded) {
				return rutils.NewModuleStartUpTimeoutError(m.cfg.Name)
			}
			return ctxTimeout.Err()
		case <-checkTicker.C:
		}
		err = modlib.CheckSocketOwner(m.addr)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}
		break
	}
	return nil
}

func (m *module) stopProcess() error {
	if m.process == nil {
		return nil
	}
	// Attempt to remove module's .sock file if module did not remove it
	// already.
	defer rutils.RemoveFileNoError(m.addr)

	// TODO(RSDK-2551): stop ignoring exit status 143 once Python modules handle
	// SIGTERM correctly.
	if err := m.process.Stop(); err != nil &&
		!strings.Contains(err.Error(), errMessageExitStatus143) {
		return err
	}
	return nil
}

func (m *module) registerResources(mgr modmaninterface.ModuleManager, logger logging.Logger) {
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
				logger.Infow("Registering component API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
				resource.RegisterComponent(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
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
		case api.API.IsService():
			for _, model := range models {
				logger.Infow("Registering service API and model from module", "module", m.cfg.Name, "API", api.API, "model", model)
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
			logger.Errorw("Invalid module type", "API type", api.API.Type)
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

func (m *module) cleanupAfterStartupFailure(logger logging.Logger) {
	if err := m.stopProcess(); err != nil {
		msg := "Error while stopping process of module that failed to start"
		logger.Errorw(msg, "module", m.cfg.Name, "error", err)
	}
	if m.sharedConn != nil {
		if err := m.sharedConn.Close(); err != nil {
			msg := "Error while closing shared connection to module that failed to start"
			logger.Errorw(msg, "module", m.cfg.Name, "error", err)
		}
	} else {
		if m.peerConn != nil {
			if err := m.peerConn.Close(); err != nil {
				logger.Debugw("error closing optional webrtc peer connection", "err", err)
			}
		}
		if err := m.conn.Close(); err != nil {
			msg := "Error while closing connection to module that failed to start"
			logger.Errorw(msg, "module", m.cfg.Name, "error", err)
		}
	}
}

func (m *module) cleanupAfterCrash(mgr *Manager) {
	if err := m.stopProcess(); err != nil {
		msg := "Error while stopping process of crashed module"
		mgr.logger.Errorw(msg, "module", m.cfg.Name, "error", err)
	}
	if m.sharedConn != nil {
		if err := m.sharedConn.Close(); err != nil {
			msg := "error while closing shared connection to crashed module"
			mgr.logger.Errorw(msg, "module", m.cfg.Name, "error", err)
		}
	} else {
		if m.peerConn != nil {
			if err := m.peerConn.Close(); err != nil {
				mgr.logger.Debugw("error closing optional webrtc peer connection", "err", err)
			}
		}
		if err := m.conn.Close(); err != nil {
			msg := "error while closing connection to crashed module"
			mgr.logger.Errorw(msg, "module", m.cfg.Name, "error", err)
		}
	}
	mgr.rMap.Range(func(r resource.Name, mod *module) bool {
		if mod == m {
			mgr.rMap.Delete(r)
		}
		return true
	})
	mgr.modules.Delete(m.cfg.Name)
}

func (m *module) getFullEnvironment(viamHomeDir string) map[string]string {
	environment := map[string]string{
		"VIAM_HOME":        viamHomeDir,
		"VIAM_MODULE_DATA": m.dataDir,
	}
	if m.cfg.Type == config.ModuleTypeRegistry {
		environment["VIAM_MODULE_ID"] = m.cfg.ModuleID
	}
	// Overwrite the base environment variables with the module's environment variables (if specified)
	for key, value := range m.cfg.Environment {
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

func generateSDP(pc *webrtc.PeerConnection) (string, error) {
	if pc == nil {
		return "", nil
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		return "", err
	}

	if err = pc.SetLocalDescription(offer); err != nil {
		return "", err
	}

	<-webrtc.GatheringCompletePromise(pc)
	return rpc.EncodeSDP(pc.LocalDescription())
}

func connect(pc *webrtc.PeerConnection, encodedAnswer string, logger logging.Logger) error {
	if pc == nil {
		return errors.New("*webrtc.PeerConnection is nil")
	}
	answer := webrtc.SessionDescription{}
	if err := rpc.DecodeSDP(encodedAnswer, &answer); err != nil {
		return err
	}

	// NOTE: (Nick S & Dan G):
	// Experimentally it appears that SetRemoteDescription does not
	// need to succeed in order for a usable peer connection to be
	// established. It is not clear to us why. We are not propagating
	// this error in order to prevent deadlocking due to the module
	// expecting the peer connection to be used & the mod manager
	// expecting it to not be used.
	// This may require more investigation.
	if err := pc.SetRemoteDescription(answer); err != nil {
		logger.Debugw("SetRemoteDescription failed with", "err", err)
		return nil
	}

	return nil
}
