// Package main is a module for testing, with an inline generic component to return internal data and perform other test functions.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

var (
	myModel = resource.NewModel("rdk", "test", "helper")
	myMod   *module.Module
)

func main() {
	utils.ContextualMain(mainWithArgs, golog.NewDevelopmentLogger("TestModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) error {
	var err error
	myMod, err = module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}
	registry.RegisterComponent(generic.Subtype, myModel, registry.Component{Constructor: newHelper})
	err = myMod.AddModelFromRegistry(ctx, generic.Subtype, myModel)
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

func newHelper(ctx context.Context, deps registry.Dependencies, cfg config.Component, logger golog.Logger) (interface{}, error) {
	return &helper{
		logger: logger,
	}, nil
}

type helper struct {
	generic.Generic
	logger golog.Logger
}

// DoCommand is the only method of this component. It looks up the "real" command from the map it's passed.
//
//nolint:unparam
func (h *helper) DoCommand(ctx context.Context, req map[string]interface{}) (map[string]interface{}, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	switch req["command"] {
	case "sleep":
		time.Sleep(time.Second * 1)
		//nolint:nilnil
		return nil, nil
	case "get_ops":
		ops := myMod.OperationManager().All()
		var opsOut []string
		for _, op := range ops {
			opsOut = append(opsOut, op.ID.String())
		}
		return map[string]interface{}{"ops": opsOut}, nil
	default:
		return nil, fmt.Errorf("unknown command string %s", cmd)
	}
}
