// Package modmanager provides the module manager for a robot.
package modmanager

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils/pexec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	modlib "go.viam.com/rdk/module"
	modif "go.viam.com/rdk/module/modmanager/modmaninterface"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// NewManager returns a Manager.
func NewManager(r robot.LocalRobot) (modif.ModuleManager, error) {
	return &Manager{
		logger:  r.Logger().Named("modmanager"),
		modules: map[string]*module{},
		r:       r,
		rMap:    map[resource.Name]*module{},
	}, nil
}

type module struct {
	name    string
	process pexec.ManagedProcess
	handles modlib.HandlerMap
	conn    *grpc.ClientConn
	client  pb.ModuleServiceClient
	addr    string
}

// Manager is the root structure for the module system.
type Manager struct {
	mu      sync.RWMutex
	logger  golog.Logger
	modules map[string]*module
	r       robot.LocalRobot
	rMap    map[resource.Name]*module
}

// Close terminates module connections and processes.
func (mgr *Manager) Close(ctx context.Context) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	var err error
	for _, mod := range mgr.modules {
		if mod.conn != nil {
			err = multierr.Combine(err, mod.conn.Close())
		}
		if mod.process != nil {
			err = multierr.Combine(err, mod.process.Stop())
		}
	}
	return err
}

// AddModule adds and starts a new resource module.
func (mgr *Manager) AddModule(ctx context.Context, cfg config.Module) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	_, exists := mgr.modules[cfg.Name]
	if exists {
		return nil
	}

	mod := &module{}
	mgr.modules[cfg.Name] = mod

	parentAddr, err := mgr.r.LiteAddress()
	if err != nil {
		return err
	}
	mod.addr = filepath.Dir(parentAddr) + "/" + cfg.Name + ".sock"

	pcfg := pexec.ProcessConfig{
		ID:   cfg.Name,
		Name: cfg.ExePath,
		Args: []string{mgr.modules[cfg.Name].addr},
		Log:  true,
	}
	mod.process = pexec.NewManagedProcess(pcfg, mgr.logger)

	err = mod.process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	for {
		select {
		case <-ctxTimeout.Done():
			return errors.Errorf("timed out waiting for module %s to start listening", mod.name)
		default:
		}
		err = modlib.CheckSocketOwner(mod.addr)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return errors.WithMessage(err, "module startup failed")
		}
		break
	}

	conn, err := grpc.Dial(
		"unix://"+mod.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	mod.conn = conn
	mod.client = pb.NewModuleServiceClient(conn)

	err = mod.checkReady(ctx, parentAddr)
	if err != nil {
		return errors.WithMessage(err, "error while waiting for module to start"+mod.name)
	}

	for api, models := range mod.handles {
		known := registry.ResourceSubtypeLookup(api.Subtype)
		if known == nil {
			registry.RegisterResourceSubtype(api.Subtype, registry.ResourceSubtype{ReflectRPCServiceDesc: api.Desc})
		}

		switch api.Subtype.ResourceType {
		case resource.ResourceTypeComponent:
			for _, model := range models {
				registry.RegisterComponent(api.Subtype, model, registry.Component{
					Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
						return mgr.AddComponent(ctx, cfg, DepsToNames(deps))
					},
				})
			}
		case resource.ResourceTypeService:
			for _, model := range models {
				registry.RegisterService(api.Subtype, model, registry.Service{
					Constructor: func(ctx context.Context, r robot.Robot, cfg config.Service, logger golog.Logger) (interface{}, error) {
						return mgr.AddService(ctx, cfg)
					},
				})
			}
		default:
			mgr.logger.Errorf("invalid module type: %s", api.Subtype.Type)
		}
	}

	return nil
}

// AddComponent tells a component module to configure a new component.
func (mgr *Manager) AddComponent(ctx context.Context, cfg config.Component, deps []string) (interface{}, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	module := mgr.getComponentModule(cfg)
	if module == nil {
		return nil, errors.Errorf("no module registered to serve resource api %s and model %s", cfg.ResourceName().Subtype, cfg.Model)
	}

	cfgProto, err := config.ComponentConfigToProto(&cfg)
	if err != nil {
		return nil, err
	}
	_, err = module.client.AddComponent(ctx, &pb.AddComponentRequest{Config: cfgProto, Dependencies: deps})
	if err != nil {
		return nil, err
	}
	mgr.rMap[cfg.ResourceName()] = module

	c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
	if c == nil || c.RPCClient == nil {
		mgr.logger.Warnf("no known grpc client for modular resource %s", cfg.ResourceName())
		return rdkgrpc.NewForeignResource(cfg.ResourceName(), module.conn), nil
	}
	nameR := cfg.ResourceName().ShortName()
	return c.RPCClient(ctx, module.conn, nameR, mgr.logger), nil
}

// AddService tells a service module to configure a new service.
func (mgr *Manager) AddService(ctx context.Context, cfg config.Service) (interface{}, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	for _, module := range mgr.modules {
		var api resource.RPCSubtype
		var ok bool
		for a := range module.handles {
			if a.Subtype == cfg.ResourceName().Subtype {
				api = a
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		for _, model := range module.handles[api] {
			if cfg.Model == model {
				cfgProto, err := config.ServiceConfigToProto(&cfg)
				if err != nil {
					return nil, err
				}
				_, err = module.client.AddService(ctx, &pb.AddServiceRequest{Config: cfgProto})
				if err != nil {
					return nil, err
				}
				mgr.rMap[cfg.ResourceName()] = module

				c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
				if c == nil || c.RPCClient == nil {
					mgr.logger.Warnf("no known grpc client for modular resource %s", cfg.ResourceName())
					return rdkgrpc.NewForeignResource(cfg.ResourceName(), module.conn), nil
				}
				nameR := cfg.ResourceName().ShortName()
				return c.RPCClient(ctx, module.conn, nameR, mgr.logger), nil
			}
		}
	}
	return nil, errors.Errorf("no module registered to serve resource api %s and model %s", cfg.ResourceName().Subtype, cfg.Model)
}

// IsModularComponent returns true if a component would be handled by a module.
func (mgr *Manager) IsModularComponent(cfg config.Component) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.getComponentModule(cfg) != nil
}

// ReconfigureComponent updates/reconfigures a modular component with a new configuration.
func (mgr *Manager) ReconfigureComponent(ctx context.Context, cfg config.Component, deps []string) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	module := mgr.getComponentModule(cfg)
	if module == nil {
		return errors.Errorf("no module registered to serve resource api %s and model %s", cfg.ResourceName().Subtype, cfg.Model)
	}

	cfgProto, err := config.ComponentConfigToProto(&cfg)
	if err != nil {
		return err
	}
	_, err = module.client.ReconfigureComponent(ctx, &pb.ReconfigureComponentRequest{Config: cfgProto, Dependencies: deps})
	if err != nil {
		return err
	}

	return nil
}

// IsModularService returns true if a service would be handled by a module.
func (mgr *Manager) IsModularService(cfg config.Service) bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.getServiceModule(cfg) != nil
}

// ReconfigureService updates/reconfigures a modular service with a new configuration.
func (mgr *Manager) ReconfigureService(ctx context.Context, cfg config.Service) error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	module := mgr.getServiceModule(cfg)
	if module == nil {
		return errors.Errorf("no module registered to serve resource api %s and model %s", cfg.ResourceName().Subtype, cfg.Model)
	}

	cfgProto, err := config.ServiceConfigToProto(&cfg)
	if err != nil {
		return err
	}
	_, err = module.client.ReconfigureService(ctx, &pb.ReconfigureServiceRequest{Config: cfgProto})
	if err != nil {
		return err
	}

	return nil
}

// IsModularResource returns true if a component is handled by a module.
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

	_, err := module.client.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: name.String()})
	return err
}

func (mgr *Manager) getComponentModule(cfg config.Component) *module {
	for _, module := range mgr.modules {
		var api resource.RPCSubtype
		var ok bool
		for a := range module.handles {
			if a.Subtype == cfg.ResourceName().Subtype {
				api = a
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		for _, model := range module.handles[api] {
			if cfg.Model == model {
				return module
			}
		}
	}
	return nil
}

func (mgr *Manager) getServiceModule(cfg config.Service) *module {
	for _, module := range mgr.modules {
		var api resource.RPCSubtype
		var ok bool
		for a := range module.handles {
			if a.Subtype == cfg.ResourceName().Subtype {
				api = a
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		for _, model := range module.handles[api] {
			if cfg.Model == model {
				return module
			}
		}
	}
	return nil
}

// DepsToNames converts a dependency list to a simple string slice.
func DepsToNames(deps registry.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}

func (m *module) checkReady(ctx context.Context, addr string) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, time.Second*10)
	defer cancelFunc()
	for {
		req := &pb.ReadyRequest{ParentAddress: addr}
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
