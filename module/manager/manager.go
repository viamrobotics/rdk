// Package manager provides the module manager for a robot.
package manager

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/rdk/config"
	rdkgrpc "go.viam.com/rdk/grpc"
	modlib "go.viam.com/rdk/module"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// NewManager returns a Manager.
func NewManager(r robot.LocalRobot) (*Manager, error) {
	mgr := &Manager{
		mu:         sync.RWMutex{},
		logger:     r.Logger(),
		modules:    map[string]*module{},
		serviceMap: map[resource.Name]rpc.ClientConn{},
		r:          r,
	}
	return mgr, nil
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
	mu         sync.RWMutex
	logger     golog.Logger
	modules    map[string]*module
	serviceMap map[resource.Name]rpc.ClientConn
	r          robot.LocalRobot
}

// Stop signals logic modules to stop operation and release resources.
// Should be called before Close().
func (mgr *Manager) Stop(ctx context.Context) error {
	// TODO (@Otterverse) stop logic and services.
	return nil
}

// Close terminates module connections and processes.
// Should only be called after Stop().
func (mgr *Manager) Close(ctx context.Context) error {
	var err error
	for _, mod := range mgr.modules {
		err = multierr.Combine(
			err,
			mod.conn.Close(),
			err, mod.process.Stop(),
			os.RemoveAll(filepath.Dir(mod.addr)),
		)
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

	dir, err := os.MkdirTemp("", "viam-module-*")
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}
	mod.addr = dir + "/module.sock"

	pcfg := pexec.ProcessConfig{
		ID:   cfg.Name,
		Name: cfg.Path,
		Args: []string{mgr.modules[cfg.Name].addr},
		Log:  true,
		// CWD string
		// OneShot bool
	}
	mod.process = pexec.NewManagedProcess(pcfg, mgr.logger)

	err = mod.process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
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

	parentAddr, err := mgr.r.LiteAddress()
	if err != nil {
		return err
	}
	err = mod.checkReady(ctx, parentAddr)
	if err != nil {
		return errors.WithMessage(err, "error while waiting for module to start"+mod.name)
	}

	for api, models := range mod.handles {
		known := registry.RegisteredResourceSubtypes()
		_, ok := known[api.Subtype]
		if !ok {
			registry.RegisterResourceSubtype(api.Subtype, registry.ResourceSubtype{
				ReflectRPCServiceDesc: api.Desc,
				Foreign:               true,
			})
		}

		switch api.Subtype.ResourceType {
		case resource.ResourceTypeComponent:
			for _, model := range models {
				registry.RegisterComponent(api.Subtype, model, registry.Component{
					Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
						return mgr.AddComponent(ctx, cfg, depsToNames(deps))
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
		case resource.TypeName("logic"):
			mgr.logger.Warn("modular logic not yet supported")
		default:
			mgr.logger.Errorf("invalid module type: %s", api.Subtype.Type)
		}
	}

	return nil
}

// AddComponent tells a component module to configure a new component.
func (mgr *Manager) AddComponent(ctx context.Context, cfg config.Component, deps []string) (interface{}, error) {
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
				cfgProto, err := config.ComponentConfigToProto(&cfg)
				if err != nil {
					return nil, err
				}
				_, err = module.client.AddComponent(ctx, &pb.AddComponentRequest{Config: cfgProto, Dependencies: deps})
				if err != nil {
					return nil, err
				}
				mgr.serviceMap[cfg.ResourceName()] = module.conn

				c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
				if c == nil || c.RPCClient == nil {
					mgr.logger.Warnf("no known grpc client for modular resource %s", cfg.ResourceName())
					return rdkgrpc.NewForeignResource(cfg.ResourceName(), module.conn), nil
				}
				nameR := cfg.ResourceName().ShortName()
				resourceClient := c.RPCClient(ctx, module.conn, nameR, mgr.logger)
				if c.Reconfigurable == nil {
					return resourceClient, nil
				}
				return c.Reconfigurable(resourceClient)
			}
		}
	}
	return nil, errors.Errorf("no module registered to serve resource api %s and model %s", cfg.ResourceName().Subtype, cfg.Model)
}

// AddService tells a service module to configure a new service.
func (mgr *Manager) AddService(ctx context.Context, cfg config.Service) (interface{}, error) {
	// TODO (@Otterverse) add service support
	return nil, errors.New("service handling not yet implemented")
}

func depsToNames(deps registry.Dependencies) []string {
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
		// TODO (@Otterverse) test if this actually fails on context.Done()
		req := &pb.ReadyRequest{ParentAddress: addr}
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
