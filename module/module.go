// Package module provides services for external resource and logic modules.
package module

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.viam.com/utils/pexec"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"

	pb "go.viam.com/rdk/proto/api/module/v1"
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

type Manager struct {
	mu         sync.RWMutex
	logger     golog.Logger
	modules    map[string]*module
	serviceMap map[resource.Name]rpc.ClientConn
}

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
		// CWD     string   `json:"cwd"`
		// OneShot bool     `json:"one_shot"`
		// Log     bool     `json:"log"`
	}
	mgr.modules[cfg.Name].process = pexec.NewManagedProcess(pcfg, mgr.logger)

	err = mgr.modules[cfg.Name].process.Start(context.Background())
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	conn, err := grpc.Dial(
		string("unix://" + mgr.modules[cfg.Name].addr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
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
				Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
					return mgr.AddComponent(ctx, cfg)
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


	time.Sleep(time.Second * 5)


	return nil
}

func (mgr *Manager) AddComponent(ctx context.Context, cfg config.Component) (interface{}, error) {
	for _, module := range mgr.modules {
		for _, model := range module.serves {
			if cfg.Model == model {
				client := pb.NewModuleServiceClient(module.conn)
				cfgStruct, err := protoutils.StructToStructPb(cfg)
				if err != nil {
					return nil, err
				}
				req := &pb.AddResourceRequest{
					Name: protoutils.ResourceNameToProto(cfg.ResourceName()),
					Config:       cfgStruct,
				}
				_, err = client.AddResource(ctx, req)
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

				return resourceClient, nil
			}
		}
	}
	return nil, errors.Errorf("no module registered to serve resource model %s", cfg.ResourceName().Model)
}
