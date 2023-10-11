// Package main is a module with a built-in "counter" component model, that will simply track numbers.
// It uses the rdk:component:generic interface for simplicity.
package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var myModel = resource.NewModel("acme", "demo", "mycounter")

func main() {
	utils.ContextualMain(mainWithArgs, logging.NewDevelopmentLogger("SimpleModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	// Instantiate the module itself
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// We first put our component's constructor in the registry, then tell the module to load it
	// Note that all resources must be added before the module is started.
	resource.RegisterComponent(generic.API, myModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: newCounter,
	})
	err = myMod.AddModelFromRegistry(ctx, generic.API, myModel)
	if err != nil {
		return err
	}

	// The module is started.
	err = myMod.Start(ctx)
	// Close is deferred and will run automatically when this function returns.
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}

	// This will block (leaving the module running) until the context is cancelled.
	// The utils.ContextualMain catches OS signals and will cancel our context for us when one is sent for shutdown/termination.
	<-ctx.Done()
	// The deferred myMod.Close() will now run.
	return nil
}

// newCounter is used to create a new instance of our specific model. It is called for each component in the robot's config with this model.
func newCounter(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger *zap.SugaredLogger,
) (resource.Resource, error) {
	return &counter{
		Named: conf.ResourceName().AsNamed(),
	}, nil
}

// counter is the representation of this model. It holds only a "total" count.
type counter struct {
	resource.Named
	resource.TriviallyCloseable
	total int64
}

func (c *counter) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	atomic.StoreInt64(&c.total, 0)
	return nil
}

// DoCommand is the only method of this component. It looks up the "real" command from the map it's passed.
// Because of this, any arbitrary commands can be received, and any data returned.
func (c *counter) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	// We look for a map key called "command"
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	// If it's "get" we return the current total.
	if cmd == "get" {
		return map[string]interface{}{"total": atomic.LoadInt64(&c.total)}, nil
	}

	// If it's "add" we atomically add a second key "value" to the total.
	if cmd == "add" {
		_, ok := req["value"]
		if !ok {
			return nil, errors.New("value must exist")
		}
		val, ok := req["value"].(float64)
		if !ok {
			return nil, errors.New("value must be a number")
		}
		atomic.AddInt64(&c.total, int64(val))
		// We return the new total after the addition.
		return map[string]interface{}{"total": atomic.LoadInt64(&c.total)}, nil
	}
	// The command must've been something else.
	return nil, fmt.Errorf("unknown command string %s", cmd)
}
