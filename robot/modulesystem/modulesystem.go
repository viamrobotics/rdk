// The module system provides service for external resource and logic modules.
package modulesystem

import (
	"context"
	"sync"
	"github.com/edaniels/golog"

	"go.uber.org/multierr"

	"go.viam.com/utils/pexec"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("module")

// A Service that handles external resource modules.
type Service interface {
	RegisterModule(ctx context.Context, path string, res resource.Name) error
}

// New returns a module system service for the given robot.
func New(ctx context.Context, r robot.Robot, logger golog.Logger) Service {
	return &moduleService{
		robot:   r,
		logger:  logger,
		modules: map[string]module{},
	}
}

// the module system.
type moduleService struct {
	mu             sync.RWMutex
	robot          robot.Robot
	logger         golog.Logger
	modules        map[string]module
}

type module struct {
	process pexec.ManagedProcess
	serves  []resource.Name
	addr    string
}

func (svc *moduleService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	return nil
}

func (svc *moduleService) Close(ctx context.Context) error {
	var err error
	for _, mod := range svc.modules {
		multierr.Combine(err, mod.process.Stop())
	}
	return err
}

func (svc *moduleService) RegisterModule(ctx context.Context, path string, res resource.Name) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	mod, exists := svc.modules[path]
	if exists {
		mod.serves = append(mod.serves, res)
		return nil
	}

	cfg := pexec.ProcessConfig{
		ID:   path,
		Name: path,
		// Args    []string `json:"args"`
		// CWD     string   `json:"cwd"`
		// OneShot bool     `json:"one_shot"`
		// Log     bool     `json:"log"`
	}

	proc := pexec.NewManagedProcess(cfg, svc.logger)
	svc.modules[path] = module{
		process: proc,
		serves: []resource.Name{res},
	}
	return proc.Start(ctx)
}
