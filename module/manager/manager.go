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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"

	pb "go.viam.com/api/proto/viam/module/v1"
)

// NewManager returns a Manager
func NewManager(cfgs []config.Module, logger golog.Logger) (*Manager, error) {
	mgr := &Manager{
		mu:         sync.RWMutex{},
		logger:     logger,
		modules:    map[string]*module{},
		serviceMap: map[resource.Name]rpc.ClientConn{},
	}

	for _, mod := range cfgs {
		err := mgr.AddModule(mod)
		logger.Debugf("SMURF98: %+v", mod.Models)
		if err != nil {
			return nil, err
		}
	}

	return mgr, nil
}

type module struct {
	name    string
	process pexec.ManagedProcess
	serves  []resource.Model
	conn    rpc.ClientConn
	addr    string
}

// Manager is the root structure for the module system.
type Manager struct {
	mu         sync.RWMutex
	logger     golog.Logger
	modules    map[string]*module
	serviceMap map[resource.Name]rpc.ClientConn
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
			os.RemoveAll(filepath.Dir(string(mod.addr))),
		)
	}
	return err
}

// AddModule adds and starts a new resource module.
func (mgr *Manager) AddModule(cfg config.Module) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	_, exists := mgr.modules[cfg.Name]
	if exists {
		return nil
	}
	mgr.modules[cfg.Name] = &module{}

	dir, err := os.MkdirTemp("", "viam-module-*")
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}
	mgr.modules[cfg.Name].addr = dir + "/module.sock"

	pcfg := pexec.ProcessConfig{
		ID:   string(cfg.Name),
		Name: string(cfg.Path),
		Args: []string{ string(mgr.modules[cfg.Name].addr) },
		Log: true,
		// CWD string
		// OneShot bool
	}
	mgr.modules[cfg.Name].process = pexec.NewManagedProcess(pcfg, mgr.logger)

	err = mgr.modules[cfg.Name].process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	conn, err := grpc.Dial(
		string("unix://" + mgr.modules[cfg.Name].addr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor()),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor()),
	)
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}
	mgr.modules[cfg.Name].conn = conn
	mgr.modules[cfg.Name].serves = cfg.Models

	for _, model := range cfg.Models {
		switch cfg.Type {
		case "component":
			registry.RegisterComponent(generic.Subtype, model, registry.Component{
				Constructor: func(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
					return mgr.AddComponent(ctx, cfg, depsToNames(deps))
				},
			})
		case "service":
			mgr.logger.Warn("modular services not yet supported")
		case "logic":
			mgr.logger.Warn("modular logic not yet supported")
		default:
			mgr.logger.Errorf("invalid module type: %s", cfg.Type)
		}
	}

	return nil
}

// AddComponent tells a component module to configure a new component.
func (mgr *Manager) AddComponent(ctx context.Context, cfg config.Component, deps []string) (interface{}, error) {
	for _, module := range mgr.modules {
		for _, model := range module.serves {
			if cfg.Model == model {
				client := pb.NewModuleServiceClient(module.conn)
				err := ping(ctx, client)
				if err != nil {
					return nil, errors.WithMessage(err, "SMURF PING: ")
				}

				cfgProto, err := config.ComponentConfigToProto(&cfg)
				if err != nil {
					return nil, err
				}
				_, err = client.AddComponent(ctx, &pb.AddComponentRequest{Config: cfgProto, Dependencies: deps})
				if err != nil {
					return nil, err
				}
				mgr.serviceMap[cfg.ResourceName()] = module.conn

				c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
				nameR := cfg.ResourceName().ShortName()
				// TODO SMURF proper context
				resourceClient := c.RPCClient(ctx, module.conn, nameR, mgr.logger)
				if c.Reconfigurable == nil {
					return resourceClient, nil
				}
				return c.Reconfigurable(resourceClient)
			}
		}
	}
	return nil, errors.Errorf("no module registered to serve resource model %s", cfg.ResourceName().Model)
}

// AddService tells a service module to configure a new service.
func (mgr *Manager) AddService(ctx context.Context, cfg config.Service) (interface{}, error) {
	// TODO (@Otterverse) add service support
	return nil, nil
}

func depsToNames (deps registry.Dependencies) []string {
	var depStrings []string
	for dep := range deps {
		depStrings = append(depStrings, dep.String())
	}
	return depStrings
}

func ping(ctx context.Context, client pb.ModuleServiceClient) error {
	ctxTimeout, cancelFunc := context.WithTimeout(ctx, time.Second * 10)
	defer cancelFunc()
	for {
		// TODO (@Otterverse) test if this actually fails on context.Done()
		resp, err := client.Ready(ctxTimeout, &pb.ReadyRequest{}, grpc_retry.WithMax(5000))
		if err != nil {
			return err
		}

		if resp.Ready {
			return nil
		}
	}
}