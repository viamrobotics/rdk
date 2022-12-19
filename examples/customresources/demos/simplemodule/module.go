// Package main is a module with a built-in "counter" component model, that will simply track numbers.
// It uses the rdk:component:generic interface for simplicity.
package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"

	"go.viam.com/utils"
)
var myModel = resource.NewModel("acme", "demo","mycounter")

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewDevelopmentLogger("SimpleModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	// Instantiate the module itself
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	// We first put our component's constructor in the registry, then tell the module to load it
	// Note that all resources must be added before the module is started.
	registry.RegisterComponent(generic.Subtype, myModel, registry.Component{Constructor: newCounter})
	myMod.AddModelFromRegistry(ctx, generic.Subtype, myModel)

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
func newCounter(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger *zap.SugaredLogger) (interface{}, error) {
	return &counter{}, nil
}

// counter is the representation of this model. It holds only a "total" count.
type counter struct {
	total int64
}

// DoCommand is the only method of this component. It looks up the "real" command from the map it's passed.
// Because of this, any arbitrary commands can be recieved, and any data returned.
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
