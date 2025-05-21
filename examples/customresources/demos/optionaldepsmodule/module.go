// Package main is a module that utilizes both required and optional implicit
// dependencies. It serves a generic component that has a required dependency on one motor
// and an optional dependency on another motor.
package main

import (
	"context"
	"errors"
	"fmt"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var model = resource.NewModel("acme", "demo", "foo")

func main() {
	resource.RegisterComponent(generic.API, model, resource.Registration[resource.Resource, *FooConfig]{
		Constructor: newFoo,
	})

	module.ModularMain(resource.APIModel{generic.API, model})
}

// FooConfig contains a required and optional motor that the component will
// necessarily and optionally depend upon.
type FooConfig struct {
	RequiredMotor string `json:"required_motor"`
	OptionalMotor string `json:"optional_motor"`
}

// Validate validates the config and returns a required dependency on
// `required_motor` and an optional dependency on `optional_motor`.
func (fCfg *FooConfig) Validate(path string) ([]string, []string, error) {
	var requiredDeps, optionalDeps []string

	if fCfg.RequiredMotor == "" {
		return nil, nil,
			fmt.Errorf(`expected "required_motor" attribute for foo %q`, path)
	}
	requiredDeps = append(requiredDeps, fCfg.RequiredMotor)

	if fCfg.OptionalMotor != "" {
		optionalDeps = append(optionalDeps, fCfg.OptionalMotor)
	}

	return requiredDeps, optionalDeps, nil
}

type foo struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger

	requiredMotor motor.Motor
	optionalMotor motor.Motor
}

func newFoo(ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	f := &foo{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}

	if err := f.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return f, nil
}

func (f *foo) Reconfigure(ctx context.Context, deps resource.Dependencies,
	conf resource.Config,
) error {
	fooConfig, err := resource.NativeConfig[*FooConfig](conf)
	if err != nil {
		return err
	}

	f.requiredMotor, err = motor.FromDependencies(deps, fooConfig.RequiredMotor)
	if err != nil {
		return fmt.Errorf("could not get required motor %s from dependencies",
			fooConfig.RequiredMotor)
	}

	f.optionalMotor, err = motor.FromDependencies(deps, fooConfig.OptionalMotor)
	if err != nil {
		f.logger.Infof("could not get optional motor %s from dependencies; continuing",
			fooConfig.OptionalMotor)
	}

	return nil
}

// DoCommand is the only method of this component; shows how one might leverage the
// required and optional motor dependencies.
func (f *foo) DoCommand(ctx context.Context, req map[string]any) (map[string]any, error) {
	cmd, ok := req["command"]
	if !ok {
		return nil, errors.New("missing 'command' string")
	}

	// "required_motor_state" will check the state of the required motor.
	if cmd == "required_motor_state" {
		if f.requiredMotor == nil {
			return map[string]any{"required_motor_state": "unset"}, nil
		}

		requiredMotorIsMoving, err := f.requiredMotor.IsMoving(ctx)
		if err != nil {
			return map[string]any{"required_motor_state": "unreachable"}, nil //nolint:nilerr
		}
		return map[string]any{"required_motor_state": fmt.Sprintf("moving: %v", requiredMotorIsMoving)}, nil
	}

	// "optional_motor_state" will check the state of the optional motor.
	if cmd == "optional_motor_state" {
		if f.optionalMotor == nil {
			return map[string]any{"optional_motor_state": "unset"}, nil
		}

		optionalMotorIsMoving, err := f.optionalMotor.IsMoving(ctx)
		if err != nil {
			return map[string]any{"optional_motor_state": "unreachable"}, nil //nolint:nilerr
		}
		return map[string]any{"optional_motor_state": fmt.Sprintf("moving: %v", optionalMotorIsMoving)}, nil
	}

	// The command must've been something else (unrecognized).
	return nil, fmt.Errorf("unknown command string %s", cmd)
}
