// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.uber.org/zap/zapcore"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/pexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
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
)

// NewManager returns a Manager.
func NewManager(parentAddr string, logger golog.Logger, options modmanageroptions.Options) modmaninterface.ModuleManager {
	restartCtx, restartCtxCancel := context.WithCancel(context.Background())
	return &Manager{
		logger:                  logger,
		modules:                 map[string]*module{},
		parentAddr:              parentAddr,
		rMap:                    map[resource.Name]*module{},
		untrustedEnv:            options.UntrustedEnv,
		removeOrphanedResources: options.RemoveOrphanedResources,
		restartCtx:              restartCtx,
		restartCtxCancel:        restartCtxCancel,
	}
}

type module struct {
	name        string
	exe         string
	logLevel    string
	modType     config.ModuleType
	moduleID    string
	environment map[string]string
	process     pexec.ManagedProcess
	handles     modlib.HandlerMap
	conn        *grpc.ClientConn
	client      pb.ModuleServiceClient
	addr        string
	resources   map[resource.Name]*addedResource

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
}

type addedResource struct {
	conf resource.Config
	deps []string
}

// Manager is the root structure for the module system.
type Manager struct {
	mu                      sync.RWMutex
	logger                  golog.Logger
	modules                 map[string]*module
	parentAddr              string
	rMap                    map[resource.Name]*module
	untrustedEnv            bool
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
	for _, mod := range mgr.modules {
		err = multierr.Combine(err, mgr.remove(mod, false))
	}
	return err
}

// Add adds and starts a new resource module.
func (mgr *Manager) Add(ctx context.Context, conf config.Module) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.add(ctx, conf, nil)
}

func (mgr *Manager) add(ctx context.Context, conf config.Module, conn *grpc.ClientConn) error {
	if mgr.untrustedEnv {
		return errModularResourcesDisabled
	}

	_, exists := mgr.modules[conf.Name]
	if exists {
		return nil
	}

	mod := &module{
		name:        conf.Name,
		exe:         conf.ExePath,
		logLevel:    conf.LogLevel,
		modType:     conf.Type,
		moduleID:    conf.ModuleID,
		environment: conf.Environment,
		conn:        conn,
		resources:   map[resource.Name]*addedResource{},
	}

	// add calls startProcess, which can also be called by the OUE handler in the attemptRestart
	// call. Both of these involve owning a lock, so in unhappy cases of malformed modules
	// this can lead to a deadlock. To prevent this, we set inStartup here to indicate to
	// the OUE handler that it shouldn't act while add is still processing.
	mod.inStartup.Store(true)
	defer mod.inStartup.Store(false)

	var success bool
	defer func() {
		if !success {
			mod.cleanupAfterStartupFailure(mgr, false)
		}
	}()

	if err := mod.startProcess(mgr.restartCtx, mgr.parentAddr,
		mgr.newOnUnexpectedExitHandler(mod), mgr.logger); err != nil {
		return errors.WithMessage(err, "error while starting module "+mod.name)
	}

	// dial will re-use mod.conn if it's non-nil (module being added in a Reconfigure).
	if err := mod.dial(); err != nil {
		return errors.WithMessage(err, "error while dialing module "+mod.name)
	}

	if err := mod.checkReady(ctx, mgr.parentAddr, mgr.logger); err != nil {
		return errors.WithMessage(err, "error while waiting for module to be ready "+mod.name)
	}

	mod.registerResources(mgr, mgr.logger)
	mgr.modules[conf.Name] = mod

	success = true
	return nil
}

// Reconfigure reconfigures an existing resource module and returns the names of
// now orphaned resources.
func (mgr *Manager) Reconfigure(ctx context.Context, conf config.Module) ([]resource.Name, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, exists := mgr.modules[conf.Name]
	if !exists {
		return nil, errors.Errorf("cannot reconfigure module %s as it does not exist", conf.Name)
	}
	handledResources := mod.resources
	var handledResourceNames []resource.Name
	for name := range handledResources {
		handledResourceNames = append(handledResourceNames, name)
	}

	if err := mgr.remove(mod, true); err != nil {
		// If removal fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	if err := mgr.add(ctx, conf, mod.conn); err != nil {
		// If re-addition fails, assume all handled resources are orphaned.
		return handledResourceNames, err
	}

	// add old module process' resources to new module; warn if new module cannot
	// handle old resource and consider that resource orphaned.
	var orphanedResourceNames []resource.Name
	for name, res := range handledResources {
		if _, err := mgr.addResource(ctx, res.conf, res.deps); err != nil {
			mgr.logger.Warnf("error while re-adding resource %s to module %s: %v",
				name, conf.Name, err)
			orphanedResourceNames = append(orphanedResourceNames, name)
		}
	}
	return orphanedResourceNames, nil
}

// Remove removes and stops an existing resource module and returns the names of
// now orphaned resources.
func (mgr *Manager) Remove(modName string) ([]resource.Name, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, exists := mgr.modules[modName]
	if !exists {
		return nil, errors.Errorf("cannot remove module %s as it does not exist", modName)
	}
	handledResources := mod.resources

	// If module handles no resources, remove it now. Otherwise mark it
	// pendingRemoval for eventual removal after last handled resource has been
	// closed.
	if len(handledResources) == 0 {
		return nil, mgr.remove(mod, false)
	}

	var orphanedResourceNames []resource.Name
	for name := range handledResources {
		orphanedResourceNames = append(orphanedResourceNames, name)
	}
	mod.pendingRemoval = true
	return orphanedResourceNames, nil
}

func (mgr *Manager) remove(mod *module, reconfigure bool) error {
	// resource manager should've removed these cleanly if this isn't a reconfigure
	if !reconfigure && len(mod.resources) != 0 {
		mgr.logger.Warnw("forcing removal of module with active resources", "module", mod.name)
	}

	// need to actually close the resources within the module itself before stopping
	for res := range mod.resources {
		_, err := mod.client.RemoveResource(context.Background(), &pb.RemoveResourceRequest{Name: res.String()})
		if err != nil {
			mgr.logger.Errorw("error removing resource", "module", mod.name, "resource", res.Name, "error", err)
		}
	}

	if err := mod.stopProcess(); err != nil {
		return errors.WithMessage(err, "error while stopping module "+mod.name)
	}

	// Do not close connection if module is being reconfigured.
	if !reconfigure {
		if mod.conn != nil {
			if err := mod.conn.Close(); err != nil {
				return errors.WithMessage(err, "error while closing connection from module "+mod.name)
			}
		}
	}

	mod.deregisterResources()

	for r, m := range mgr.rMap {
		if m == mod {
			delete(mgr.rMap, r)
		}
	}
	delete(mgr.modules, mod.name)
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

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	_, err = mod.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}
	mgr.rMap[conf.ResourceName()] = mod
	mod.resources[conf.ResourceName()] = &addedResource{conf, deps}

	apiInfo, ok := resource.LookupGenericAPIRegistration(conf.API)
	if !ok || apiInfo.RPCClient == nil {
		mgr.logger.Warnf("no built-in grpc client for modular resource %s", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), mod.conn), nil
	}
	return apiInfo.RPCClient(ctx, mod.conn, "", conf.ResourceName(), mgr.logger)
}

// ReconfigureResource updates/reconfigures a modular component with a new configuration.
func (mgr *Manager) ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	mod, ok := mgr.getModule(conf)
	if !ok {
		return errors.Errorf("no module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

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
	for _, mod := range mgr.modules {
		configs = append(configs, config.Module{
			Name: mod.name, ExePath: mod.exe, LogLevel: mod.logLevel,
		})
	}
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
	_, ok := mgr.rMap[name]
	return ok
}

// RemoveResource requests the removal of a resource from a module.
func (mgr *Manager) RemoveResource(ctx context.Context, name resource.Name) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mod, ok := mgr.rMap[name]
	if !ok {
		return errors.Errorf("resource %+v not found in module", name)
	}
	delete(mgr.rMap, name)
	delete(mod.resources, name)
	_, err := mod.client.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: name.String()})
	if err != nil {
		return err
	}

	// if the module is marked for removal, actually remove it when the final resource is closed
	if mod.pendingRemoval && len(mod.resources) == 0 {
		err = multierr.Combine(err, mgr.remove(mod, false))
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

func (mgr *Manager) getModule(conf resource.Config) (*module, bool) {
	for _, mod := range mgr.modules {
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
			continue
		}
		for _, model := range mod.handles[api] {
			if conf.Model == model && !mod.pendingRemoval {
				return mod, true
			}
		}
	}
	return nil, false
}

var (
	// oueTimeout is the length of time for which an OnUnexpectedExit function
	// can execute blocking calls.
	oueTimeout = 2 * time.Minute
	// oueRestartInterval is the interval of time at which an OnUnexpectedExit
	// function can attempt to restart the module process. Multiple restart
	// attempts will use basic backoff.
	oueRestartInterval = 5 * time.Second
)

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

		// Use oueTimeout for entire attempted module restart.
		ctx, cancel := context.WithTimeout(mgr.restartCtx, oueTimeout)
		defer cancel()

		// Log error immediately, as this is unexpected behavior.
		mgr.logger.Errorw(
			"module has unexpectedly exited, attempting to restart it",
			"module", mod.name,
			"exit_code", exitCode,
		)

		// If attemptRestart returns any orphaned resource names, restart failed,
		// and we should remove orphaned resources. Since we handle process
		// restarting ourselves, return false here so goutils knows not to attempt
		// a process restart.
		if orphanedResourceNames := mgr.attemptRestart(ctx, mod); orphanedResourceNames != nil {
			if mgr.removeOrphanedResources != nil {
				mgr.removeOrphanedResources(ctx, orphanedResourceNames)
			}
			return false
		}

		// Otherwise, add old module process' resources to new module; warn if new
		// module cannot handle old resource and remove it from mod.resources.
		// Finally, handle orphaned resources.
		var orphanedResourceNames []resource.Name
		for name, res := range mod.resources {
			if _, err := mgr.addResource(ctx, res.conf, res.deps); err != nil {
				mgr.logger.Warnw("error while re-adding resource to module",
					"resource", name, "module", mod.name, "error", err)
				delete(mgr.rMap, name)
				delete(mod.resources, name)
				orphanedResourceNames = append(orphanedResourceNames, name)
			}
		}
		if mgr.removeOrphanedResources != nil {
			mgr.removeOrphanedResources(ctx, orphanedResourceNames)
		}

		mgr.logger.Infow("module successfully restarted", "module", mod.name)
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
			mod.cleanupAfterStartupFailure(mgr, true)
		}
	}()

	// No need to check mgr.untrustedEnv, as we're restarting the same
	// executable we were given for initial module addition.

	// Attempt to restart module process 3 times.
	for attempt := 1; attempt < 4; attempt++ {
		if err := mod.startProcess(mgr.restartCtx, mgr.parentAddr,
			mgr.newOnUnexpectedExitHandler(mod), mgr.logger); err != nil {
			mgr.logger.Errorf("attempt %d: error while restarting crashed module %s: %v",
				attempt, mod.name, err)
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

	// dial will re-use mod.conn; old connection can still be used when module
	// crashes.
	if err := mod.dial(); err != nil {
		mgr.logger.Errorw("error while dialing restarted module",
			"module", mod.name, "error", err)
		return orphanedResourceNames
	}

	if err := mod.checkReady(ctx, mgr.parentAddr, mgr.logger); err != nil {
		mgr.logger.Errorw("error while waiting for restarted module to be ready",
			"module", mod.name, "error", err)
		return orphanedResourceNames
	}

	mod.registerResources(mgr, mgr.logger)

	success = true
	return nil
}

// dial will use m.conn to make a new module service client or Dial m.addr if
// m.conn is nil.
func (m *module) dial() error {
	if m.conn == nil {
		// TODO(PRODUCT-343): session support probably means interceptors here
		var err error
		m.conn, err = grpc.Dial(
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
	}
	m.client = pb.NewModuleServiceClient(m.conn)
	return nil
}

func (m *module) checkReady(ctx context.Context, parentAddr string, logger golog.Logger) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, rutils.GetResourceConfigurationTimeout(logger))
	defer cancelFunc()

	for {
		req := &pb.ReadyRequest{ParentAddress: parentAddr}
		// 5000 is an arbitrarily high number of attempts (context timeout should hit long before)
		resp, err := m.client.Ready(ctxTimeout, req, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if resp.Ready {
			m.handles, err = modlib.NewHandlerMapFromProto(ctx, resp.Handlermap, m.conn)
			return err
		}
	}
}

func (m *module) startProcess(
	ctx context.Context,
	parentAddr string,
	oue func(int) bool,
	logger golog.Logger,
) error {
	var err error
	if m.addr, err = modlib.CreateSocketAddress(filepath.Dir(parentAddr), m.name); err != nil {
		return err
	}

	pconf := pexec.ProcessConfig{
		ID:               m.name,
		Name:             m.exe,
		Args:             []string{m.addr},
		Environment:      m.environment,
		Log:              true,
		OnUnexpectedExit: oue,
	}
	// Start module process with supplied log level or "debug" if none is
	// supplied and module manager has a DebugLevel logger.
	if m.logLevel != "" {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, m.logLevel))
	} else if logger.Level().Enabled(zapcore.DebugLevel) {
		pconf.Args = append(pconf.Args, fmt.Sprintf(logLevelArgumentTemplate, "debug"))
	}

	m.process = pexec.NewManagedProcess(pconf, logger)

	if err := m.process.Start(context.Background()); err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, rutils.GetResourceConfigurationTimeout(logger))
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			return errors.Errorf("timed out waiting for module %s to start listening", m.name)
		default:
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

func (m *module) registerResources(mgr modmaninterface.ModuleManager, logger golog.Logger) {
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
				logger.Debugw("registering component from module", "module", m.name, "API", api.API, "model", model)
				resource.RegisterComponent(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger golog.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
				})
			}
		case api.API.IsService():
			for _, model := range models {
				logger.Debugw("registering service from module", "module", m.name, "API", api.API, "model", model)
				resource.RegisterService(api.API, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
					Constructor: func(
						ctx context.Context,
						deps resource.Dependencies,
						conf resource.Config,
						logger golog.Logger,
					) (resource.Resource, error) {
						return mgr.AddResource(ctx, conf, DepsToNames(deps))
					},
				})
			}
		default:
			logger.Errorf("invalid module type: %s", api.API.Type)
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

func (m *module) cleanupAfterStartupFailure(mgr *Manager, afterCrash bool) {
	if err := m.stopProcess(); err != nil {
		msg := "error while stopping process of module that failed to start"
		if afterCrash {
			msg = "error while stopping process of crashed module"
		}
		mgr.logger.Errorw(msg, "module", m.name, "error", err)
	}
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			msg := "error while closing connection to module that failed to start"
			if afterCrash {
				msg = "error while closing connection to crashed module"
			}
			mgr.logger.Errorw(msg, "module", m.name, "error", err)
		}
	}

	// Remove module from rMap and mgr.modules if startup failure was after crash.
	if afterCrash {
		for r, mod := range mgr.rMap {
			if mod == m {
				delete(mgr.rMap, r)
			}
		}
		delete(mgr.modules, m.name)
	}
}

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps resource.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}
