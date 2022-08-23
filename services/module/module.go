// Package module provides services for external resource and logic modules.
package module

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("module")

type (
	moduleAddress string
	modulePath    string
)

// A Service that handles external resource modules.
type Service interface {
	AddModule(ctx context.Context, path modulePath) error
	AddModularResource(ctx context.Context, name resource.Name) error
}

// Subtype is a constant that identifies the remote control resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

type moduleConfig struct {
	path   string
	models []string
}

// Config holds the list of modules.
type Config struct {
	modules []moduleConfig
}

func init() {
	registry.RegisterService(Subtype, registry.Service{Constructor: New})
	cType := config.ServiceType(SubtypeName)
	// registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{})

	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		return config.TransformAttributeMapToStruct(&conf, attributes)
	},
		&Config{})
}

// New returns a module system service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
	return &moduleService{
		robot:      r,
		logger:     logger,
		modules:    map[modulePath]*module{},
		serviceMap: map[resource.Name]moduleAddress{},
	}, nil
}

// the module system.
type moduleService struct {
	mu         sync.RWMutex
	robot      robot.Robot
	logger     golog.Logger
	modules    map[modulePath]*module
	serviceMap map[resource.Name]moduleAddress
}

type module struct {
	process pexec.ManagedProcess
	serves  []resource.Model
	addr    moduleAddress
}

func (svc *moduleService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	return nil
}

func (svc *moduleService) Close(ctx context.Context) error {
	var err error
	for _, mod := range svc.modules {
		err = multierr.Combine(err, mod.process.Stop())
	}
	return err
}

func (svc *moduleService) AddModule(ctx context.Context, path modulePath) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	_, exists := svc.modules[path]
	if exists {
		return nil
	}

	cfg := pexec.ProcessConfig{
		ID:   string(path),
		Name: string(path),
		// Args    []string `json:"args"`
		// CWD     string   `json:"cwd"`
		// OneShot bool     `json:"one_shot"`
		// Log     bool     `json:"log"`
	}

	proc := pexec.NewManagedProcess(cfg, svc.logger)
	svc.modules[path] = &module{
		process: proc,
	}
	return proc.Start(ctx)
}

func (svc *moduleService) AddModularResource(ctx context.Context, name resource.Name) error {
	for _, module := range svc.modules {
		for _, model := range module.serves {
			if name.Model == model {
				svc.serviceMap[name] = module.addr
				return nil
			}
		}
	}
	return errors.Errorf("no module registered to serve resource model %s", name.Model)
}
