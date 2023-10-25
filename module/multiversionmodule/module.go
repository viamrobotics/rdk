// Package main is a module designed to help build tests for reconfiguration logic between module versions
package main

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var myModel = resource.NewModel("acme", "demo", "multiversionmodule")

// VERSION is set with `-ldflags "-X main.VERSION=..."`.
var VERSION string

// config is a simple config for a generic component.
type config struct {
	// Required to be non-empty for VERSION="v2", not required otherwise
	Parameter string `json:"parameter"`
	// Required to be non-empty for VERSION="v3", not required otherwise
	Motor string `json:"motor"`
}

type component struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger
	cfg    *config
}

// Validate validates the config depending on VERSION.
func (cfg *config) Validate(_ string) ([]string, error) {
	switch VERSION {
	case "v2":
		if cfg.Parameter == "" {
			return nil, errors.New("version 2 requires a parameter")
		}
	case "v3":
		if cfg.Motor == "" {
			return nil, errors.New("version 3 requires a motor")
		}
		return []string{cfg.Motor}, nil
	default:
	}
	return make([]string, 0), nil
}

func main() {
	utils.ContextualMain(mainWithArgs, logging.NewDevelopmentLogger(fmt.Sprintf("MultiVersionModule-%s", VERSION)))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	myMod, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}
	resource.RegisterComponent(generic.API, myModel, resource.Registration[resource.Resource, *config]{
		Constructor: newComponent,
	})
	err = myMod.AddModelFromRegistry(ctx, generic.API, myModel)
	if err != nil {
		return err
	}

	err = myMod.Start(ctx)
	defer myMod.Close(ctx)
	if err != nil {
		return err
	}
	<-ctx.Done()
	return nil
}

func newComponent(_ context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	newConf, err := resource.NativeConfig[*config](conf)
	if err != nil {
		return nil, errors.Wrap(err, "create component failed due to config parsing")
	}
	return &component{
		Named:  conf.ResourceName().AsNamed(),
		cfg:    newConf,
		logger: logger,
	}, nil
}

// Reconfigure swaps the config to the new conf.
func (c *component) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	newConf, err := resource.NativeConfig[*config](conf)
	if err != nil {
		return err
	}
	if VERSION == "v3" {
		// Version 3 should have a motor in the deps
		if _, err := motor.FromDependencies(deps, "motor1"); err != nil {
			return errors.Wrapf(err, "failed to resolve motor %q for version 3", "motor1")
		}
	}
	c.cfg = newConf
	return nil
}

// DoCommand does nothing for now.
func (c *component) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}
