// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
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
	pb "go.viam.com/api/module/v1"
	"go.viam.com/rdk/utils"
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
)

var (
	validateConfigTimeout       = 5 * time.Second
	errMessageExitStatus143     = "exit status 143"
	errModularResourcesDisabled = errors.New("modular resources disabled in untrusted environment")
)

// NewManager returns a Manager.
func NewManager(parentAddr string, logger golog.Logger, options modmanageroptions.Options) modmaninterface.ModuleManager {
	return &Manager{
		logger:       logger,
		modules:      map[string]*module{},
		parentAddr:   parentAddr,
		rMap:         map[resource.Name]*module{},
		untrustedEnv: options.UntrustedEnv,
	}
}

type module struct {
	name      string
	exe       string
	process   pexec.ManagedProcess
	handles   modlib.HandlerMap
	conn      *grpc.ClientConn
	client    pb.ModuleServiceClient
	addr      string
	resources map[resource.Name]*addedResource

	// inRecovery stores whether or not an OnUnexpectedExit function is trying
	// to recover a crash of this module; inRecoveryLock guards the execution of
	// an OnUnexpectedExit function for this module.
	//
	// NOTE(benjirewis): Using just an atomic boolean is not sufficient, as OUE
	// functions for the same module cannot overlap and should not continue after
	// another OUE has finished.
	inRecovery     atomic.Bool
	inRecoveryLock sync.Mutex
}

type addedResource struct {
	conf resource.Config
	deps []string
}

// Manager is the root structure for the module system.
type Manager struct {
	mu           sync.RWMutex
	logger       golog.Logger
	modules      map[string]*module
	parentAddr   string
	rMap         map[resource.Name]*module
	untrustedEnv bool
}

// Close terminates module connections and processes.
func (mgr *Manager) Close(ctx context.Context) error {
	var err error
	for _, mod := range mgr.modules {
		err = multierr.Combine(err, mgr.remove(mod, false))
	}
	return err
}

// Add adds and starts a new resource module.
func (mgr *Manager) Add(ctx context.Context, conf config.Module) error {
	return mgr.add(ctx, conf, nil)
}

func (mgr *Manager) add(ctx context.Context, conf config.Module, conn *grpc.ClientConn) error {
	if mgr.untrustedEnv {
		return errModularResourcesDisabled
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	_, exists := mgr.modules[conf.Name]
	if exists {
		return nil
	}

	mod := &module{name: conf.Name, exe: conf.ExePath, resources: map[resource.Name]*addedResource{}}
	mgr.modules[conf.Name] = mod

	if err := mod.startProcess(ctx, mgr.parentAddr, mgr.newOUE(mod), mgr.logger); err != nil {
		return errors.WithMessage(err, "error while starting module "+mod.name)
	}

	var success bool
	defer func() {
		if !success {
			if err := mod.stopProcess(); err != nil {
				mgr.logger.Error(err)
			}
		}
	}()

	// dial will re-use conn if it's non-nil (module being added in a Reconfigure).
	if err := mod.dial(conn); err != nil {
		return errors.WithMessage(err, "error while dialing module "+mod.name)
	}

	if err := mod.checkReady(ctx, mgr.parentAddr); err != nil {
		return errors.WithMessage(err, "error while waiting for module to be ready "+mod.name)
	}

	mod.registerResources(mgr, mgr.logger)

	success = true
	return nil
}

// Reconfigure reconfigures an existing resource module and returns the names of
// now orphaned resources.
func (mgr *Manager) Reconfigure(ctx context.Context, conf config.Module) ([]resource.Name, error) {
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
		if _, err := mgr.AddResource(ctx, res.conf, res.deps); err != nil {
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
	mod, exists := mgr.modules[modName]
	if !exists {
		return nil, errors.Errorf("cannot remove module %s as it does not exist", modName)
	}
	handledResources := mod.resources

	var orphanedResourceNames []resource.Name
	for name := range handledResources {
		orphanedResourceNames = append(orphanedResourceNames, name)
	}
	return orphanedResourceNames, mgr.remove(mod, false)
}

func (mgr *Manager) remove(mod *module, reconfigure bool) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

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
	module, ok := mgr.getModule(conf)
	if !ok {
		return nil, errors.Errorf("no module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return nil, err
	}

	_, err = module.client.AddResource(ctx, &pb.AddResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}
	mgr.rMap[conf.ResourceName()] = module
	module.resources[conf.ResourceName()] = &addedResource{conf, deps}

	apiInfo, ok := resource.LookupGenericAPIRegistration(conf.API)
	if !ok || apiInfo.RPCClient == nil {
		mgr.logger.Warnf("no built-in grpc client for modular resource %s", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), module.conn), nil
	}
	return apiInfo.RPCClient(ctx, module.conn, "", conf.ResourceName(), mgr.logger)
}

// ReconfigureResource updates/reconfigures a modular component with a new configuration.
func (mgr *Manager) ReconfigureResource(ctx context.Context, conf resource.Config, deps []string) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	module, ok := mgr.getModule(conf)
	if !ok {
		return errors.Errorf("no module registered to serve resource api %s and model %s", conf.API, conf.Model)
	}

	confProto, err := config.ComponentConfigToProto(&conf)
	if err != nil {
		return err
	}
	_, err = module.client.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{Config: confProto, Dependencies: deps})
	if err != nil {
		return err
	}
	module.resources[conf.ResourceName()] = &addedResource{conf, deps}

	return nil
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
	module, ok := mgr.rMap[name]
	if !ok {
		return errors.Errorf("resource %+v not found in module", name)
	}
	delete(mgr.rMap, name)
	delete(module.resources, name)
	_, err := module.client.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: name.String()})
	return err
}

// ValidateConfig determines whether the given config is valid and returns its implicit
// dependencies.
func (mgr *Manager) ValidateConfig(ctx context.Context, conf resource.Config) ([]string, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	module, ok := mgr.getModule(conf)
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

	resp, err := module.client.ValidateConfig(ctx, &pb.ValidateConfigRequest{Config: confProto})
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
	for _, module := range mgr.modules {
		var api resource.RPCAPI
		var ok bool
		for a := range module.handles {
			if a.API == conf.API {
				api = a
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		for _, model := range module.handles[api] {
			if conf.Model == model {
				return module, true
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

// newOUE returns the appropriate OnUnexpectedExit function for the passed-in
// module to include in the pexec.ProcessConfig.
func (mgr *Manager) newOUE(mod *module) func(exitCode int) bool {
	return func(exitCode int) bool {
		mod.inRecoveryLock.Lock()
		defer mod.inRecoveryLock.Unlock()
		if mod.inRecovery.Load() {
			return false
		}
		mod.inRecovery.Store(true)
		defer mod.inRecovery.Store(false)

		// Log error immediately, as this is unexpected behavior.
		mgr.logger.Errorf(
			"module %s has unexpectedly exited with exit code %d, attempting to restart it",
			mod.name,
			exitCode,
		)

		mgr.mu.Lock()

		// Use oueTimeout for entire attempted module restart.
		ctx, cancel := context.WithTimeout(context.Background(), oueTimeout)
		defer cancel()

		// Attempt to remove module's .sock file if module did not remove it
		// already.
		utils.RemoveFileNoError(mod.addr)

		var success bool
		defer func() {
			if !success {
				// Deregister module's resources, remove module, close connection and
				// release mgr lock (successful unexpected exit handling will release
				// lock later in this function) if restart fails. Process will
				// already be stopped.
				mod.deregisterResources()
				for r, m := range mgr.rMap {
					if m == mod {
						delete(mgr.rMap, r)
					}
				}
				delete(mgr.modules, mod.name)
				if mod.conn != nil {
					if err := mod.conn.Close(); err != nil {
						mgr.logger.Error(err,
							"error while closing connection from crashed module "+mod.name)
					}
				}
				mgr.mu.Unlock()

				// Finally, assume all of module's handled resources are orphaned and
				// remove them.
				var orphanedResourceNames []resource.Name
				for name := range mod.resources {
					orphanedResourceNames = append(orphanedResourceNames, name)
				}
				mgr.r.RemoveOrphanedResources(ctx, orphanedResourceNames)
			}
		}()

		// No need to check mgr.untrustedEnv, as we're restarting the same
		// executable we were given for initial module addition.

		// Attempt to restart module process 3 times.
		for attempt := 1; attempt < 4; attempt++ {
			if err := mod.startProcess(ctx, mgr.parentAddr, mgr.newOUE(mod), mgr.logger); err != nil {
				mgr.logger.Errorf("attempt %d: error while restarting crashed module %s: %v",
					attempt, mod.name, err)
				if attempt == 3 {
					// return early upon last attempt failure.
					return false
				}
			} else {
				break
			}

			// Sleep with a bit of backoff.
			time.Sleep(time.Duration(attempt) * oueRestartInterval)
		}

		defer func() {
			if !success {
				// Stop restarted module process if there are later failures.
				if err := mod.stopProcess(); err != nil {
					mgr.logger.Error(err)
				}
			}
		}()

		// dial will re-use connection; old connection can still be used when module
		// crashes.
		if err := mod.dial(mod.conn); err != nil {
			mgr.logger.Error(err, "error while dialing restarted module "+mod.name)
			return false
		}

		if err := mod.checkReady(ctx, mgr.parentAddr); err != nil {
			mgr.logger.Error(err,
				"error while waiting for restarted module to be ready "+mod.name)
			return false
		}

		// Add old module process' resources to new module; warn if new module
		// cannot handle old resource and remove now orphaned resource.
		mgr.mu.Unlock() // Release mgr lock for AddResource calls.
		var orphanedResourceNames []resource.Name
		for name, res := range mod.resources {
			if _, err := mgr.AddResource(ctx, res.conf, res.deps); err != nil {
				mgr.logger.Warnf("error while re-adding resource %s to module %s: %v",
					name, mod.name, err)
				orphanedResourceNames = append(orphanedResourceNames, name)
			}
		}
		mgr.r.RemoveOrphanedResources(ctx, orphanedResourceNames)

		// Set success to true. Since we handle process restarting ourselves,
		// return false here so goutils knows not to attempt a process restart.
		mgr.logger.Infof("module %s successfully restarted", mod.name)
		success = true
		return false
	}
}

// dial will use the passed-in connection to make a new module service client
// or Dial m.addr if the passed-in connection is nil.
func (m *module) dial(conn *grpc.ClientConn) error {
	m.conn = conn
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

func (m *module) checkReady(ctx context.Context, parentAddr string) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, time.Second*30)
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

func (m *module) startProcess(ctx context.Context, parentAddr string,
	oue func(int) bool, logger golog.Logger,
) error {
	m.addr = filepath.ToSlash(filepath.Join(filepath.Dir(parentAddr), m.name+".sock"))
	if err := modlib.CheckSocketAddressLength(m.addr); err != nil {
		return err
	}
	pconf := pexec.ProcessConfig{
		ID:               m.name,
		Name:             m.exe,
		Args:             []string{m.addr},
		Log:              true,
		OnUnexpectedExit: oue,
	}
	m.process = pexec.NewManagedProcess(pconf, logger)

	err := m.process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*30)
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
	defer utils.RemoveFileNoError(m.addr)

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
}

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps resource.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}
