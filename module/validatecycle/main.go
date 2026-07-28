// Package main is a test-only module used to prove that a modular resource
// recovers from a config cycle whose middle step fails module validation
// (valid -> invalid -> valid). Its single generic model constructs trivially;
// Validate fails iff the `bad` attribute is true.
package main

import (
	"context"
	"fmt"

	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("rdk", "test", "validatecycle")

type conf struct {
	Bad bool `json:"bad"`
}

// Validate fails iff bad==true. No dependencies.
func (c *conf) Validate(path string) ([]string, []string, error) {
	if c.Bad {
		return nil, nil, fmt.Errorf("validatecycle: config is invalid (bad=true) at %q", path)
	}
	return nil, nil, nil
}

func main() {
	goutils.ContextualMainWithSIGPIPE(mainWithArgs, module.NewLoggerFromArgs("ValidateCycleModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	myMod, err := module.NewModuleFromArgs(ctx)
	if err != nil {
		return err
	}

	resource.RegisterComponent(generic.API, model,
		resource.Registration[resource.Resource, *conf]{Constructor: newThing})
	if err := myMod.AddModelFromRegistry(ctx, generic.API, model); err != nil {
		return err
	}

	if err := myMod.Start(ctx); err != nil {
		return err
	}
	defer myMod.Close(ctx)
	<-ctx.Done()
	return nil
}

func newThing(
	ctx context.Context,
	deps resource.Dependencies,
	c resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	return &thing{Named: c.ResourceName().AsNamed()}, nil
}

type thing struct {
	resource.Named
	resource.AlwaysRebuild
}

func (t *thing) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (t *thing) Close(ctx context.Context) error {
	return nil
}
