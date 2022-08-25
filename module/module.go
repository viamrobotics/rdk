// Package module provides services for external resource and logic modules.
package module

import (
	"context"
	"encoding/json"
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
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"

	pb "go.viam.com/rdk/proto/api/module/v1"
)

// New returns a module system service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, utils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}

	svc := &moduleService{
		mu:         sync.RWMutex{},
		robot:      r,
		logger:     logger,
		modules:    map[modulePath]*module{},
		serviceMap: map[resource.Name]rpc.ClientConn{},
	}

	for _, mod := range svcConfig.Modules {
		err := svc.AddModule(ctx, mod.Path, mod.Type, mod.Models)
		logger.Debugf("SMURF98: %+v", mod.Models)
		if err != nil {
			return nil, err
		}
	}

	return svc, nil
}

var (
	mapMu      sync.RWMutex
	logger     golog.Logger
	modulesMap map[string]*Module
	serviceMap map[resource.Name]rpc.ClientConn
)

type Module struct {
	process pexec.ManagedProcess
	serves  []resource.Model
	conn    rpc.ClientConn
	addr    string
}

func (svc *moduleService) Close(ctx context.Context) error {
	var err error
	for _, mod := range svc.modules {
		err = multierr.Combine(err, mod.conn.Close())
		err = multierr.Combine(err, mod.process.Stop())
		err = multierr.Combine(err, os.RemoveAll(filepath.Dir(string(mod.addr))))
	}
	return err
}

func (svc *moduleService) AddModule(ctx context.Context, path modulePath, modtype string, serves []resource.Model) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	_, exists := svc.modules[path]
	if exists {
		return nil
	}
	svc.modules[path] = &module{}

	dir, err := os.MkdirTemp("", "viam-module-*")
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}
	svc.modules[path].addr = moduleAddress(dir + "/module.sock")

	cfg := pexec.ProcessConfig{
		ID:   string(path),
		Name: string(path),
		Args: []string{ string(svc.modules[path].addr) },
		// CWD     string   `json:"cwd"`
		// OneShot bool     `json:"one_shot"`
		// Log     bool     `json:"log"`
	}
	svc.modules[path].process = pexec.NewManagedProcess(cfg, svc.logger)

	err = svc.modules[path].process.Start(ctx)
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}

	conn, err := grpc.Dial(
		string("unix://" + svc.modules[path].addr),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return errors.WithMessage(err, "module startup failed")
	}
	svc.modules[path].conn = conn
	svc.modules[path].serves = serves

	for _, model := range serves {
		if modtype == "component" {
			registry.RegisterComponent(generic.Subtype, model, registry.Component{
				Constructor: func(ctx context.Context, _ registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
					return svc.AddModularComponent(ctx, cfg)
				},
			})
		} else if modtype == "service" {
			svc.logger.Warn("modular services not yet supported")
		} else if modtype == "logic" {
			svc.logger.Warn("modular logic not yet supported")
		} else {
			svc.logger.Errorf("invalid module type: %s", modtype)
		}
	}


	time.Sleep(time.Second * 5)


	return nil
}

func (svc *moduleService) AddModularComponent(ctx context.Context, cfg config.Component) (interface{}, error) {
	for _, module := range svc.modules {
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
				svc.serviceMap[cfg.ResourceName()] = module.conn

				c := registry.ResourceSubtypeLookup(cfg.ResourceName().Subtype)
				nameR := cfg.ResourceName().ShortName()
				// TODO SMURF proper context
				resourceClient := c.RPCClient(ctx, module.conn, nameR, svc.logger)
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
