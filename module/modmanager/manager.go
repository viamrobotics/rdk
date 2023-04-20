// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
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
	"go.viam.com/rdk/robot"
)

var (
	validateConfigTimeout       = 5 * time.Second
	errMessageExitStatus143     = "exit status 143"
	errModularResourcesDisabled = errors.New("modular resources disabled in untrusted environment")
)

// NewManager returns a Manager.
func NewManager(r robot.LocalRobot, options modmanageroptions.Options) (modmaninterface.ModuleManager, error) {
	return &Manager{
		logger:       r.Logger().Named("modmanager"),
		modules:      map[string]*module{},
		r:            r,
		rMap:         map[resource.Name]*module{},
		untrustedEnv: options.UntrustedEnv,
	}, nil
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
	r            robot.LocalRobot
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

	parentAddr, err := mgr.r.ModuleAddress()
	if err != nil {
		return err
	}

	if err := mod.startProcess(ctx, parentAddr, mgr.logger); err != nil {
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

	if err := mod.checkReady(ctx, parentAddr); err != nil {
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

	subtypeInfo, ok := resource.LookupGenericSubtypeRegistration(conf.API)
	if !ok || subtypeInfo.RPCClient == nil {
		mgr.logger.Warnf("no built-in grpc client for modular resource %s", conf.ResourceName())
		return rdkgrpc.NewForeignResource(conf.ResourceName(), module.conn), nil
	}
	return subtypeInfo.RPCClient(ctx, module.conn, conf.ResourceName(), mgr.logger)
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
		var api resource.RPCSubtype
		var ok bool
		for a := range module.handles {
			if a.Subtype == conf.API {
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

func (m *module) startProcess(ctx context.Context, parentAddr string, logger golog.Logger) error {
	m.addr = filepath.ToSlash(filepath.Join(filepath.Dir(parentAddr), m.name+".sock"))
	if err := modlib.CheckSocketAddressLength(m.addr); err != nil {
		return err
	}
	pconf := pexec.ProcessConfig{
		ID:   m.name,
		Name: m.exe,
		Args: []string{m.addr},
		Log:  true,
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
	defer utils.UncheckedErrorFunc(func() error {
		// Attempt to remove module's .sock file if module did not remove it
		// already.
		if _, err := os.Stat(m.addr); err == nil {
			return os.Remove(m.addr)
		}
		return nil
	})

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
		if _, ok := resource.LookupGenericSubtypeRegistration(api.Subtype); !ok {
			resource.RegisterSubtype(
				api.Subtype,
				resource.SubtypeRegistration[resource.Resource]{ReflectRPCServiceDesc: api.Desc},
			)
		}

		switch api.Subtype.ResourceType {
		case resource.ResourceTypeComponent:
			for _, model := range models {
				resource.RegisterComponent(api.Subtype, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
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
		case resource.ResourceTypeService:
			for _, model := range models {
				resource.RegisterService(api.Subtype, model, resource.Registration[resource.Resource, resource.NoNativeConfig]{
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
			logger.Errorf("invalid module type: %s", api.Subtype.Type)
		}
	}
}

func (m *module) deregisterResources() {
	for api, models := range m.handles {
		for _, model := range models {
			resource.Deregister(api.Subtype, model)
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
