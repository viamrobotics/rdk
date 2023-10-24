// Package main is a module for testing, with an inline generic component to return internal data and perform other test functions.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

var (
	helperModel    = resource.NewModel("rdk", "test", "helper")
	testMotorModel = resource.NewModel("rdk", "test", "motor")
	myMod          *module.Module
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("TestModule"))
}

func mainWithArgs(ctx context.Context, args []string, logger logging.Logger) error {
	logger.Debug("debug mode enabled")

	var err error
	myMod, err = module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		generic.API,
		helperModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: newHelper})
	err = myMod.AddModelFromRegistry(ctx, generic.API, helperModel)
	if err != nil {
		return err
	}

	resource.RegisterComponent(
		motor.API,
		testMotorModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: newTestMotor})
	err = myMod.AddModelFromRegistry(ctx, motor.API, testMotorModel)
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

func newHelper(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	return &helper{
		Named:  conf.ResourceName().AsNamed(),
		logger: logging.FromZapCompatible(logger),
	}, nil
}

type helper struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

// DoCommand is the only method of this component. It looks up the "real" command from the map it's passed.
//

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
	case "echo":
		return req, nil
	case "kill_module":
		os.Exit(1)
		// unreachable return statement needed for compilation
		return nil, errors.New("unreachable error")
	default:
		return nil, fmt.Errorf("unknown command string %s", cmd)
	}
}

func newTestMotor(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	return &testMotor{
		Named: conf.ResourceName().AsNamed(),
	}, nil
}

type testMotor struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
}

var _ motor.Motor = &testMotor{}

// SetPower trivially implements motor.Motor.
func (tm *testMotor) SetPower(_ context.Context, _ float64, _ map[string]interface{}) error {
	return nil
}

// GoFor trivially implements motor.Motor.
func (tm *testMotor) GoFor(_ context.Context, _, _ float64, _ map[string]interface{}) error {
	return nil
}

// GoTo trivially implements motor.Motor.
func (tm *testMotor) GoTo(_ context.Context, _, _ float64, _ map[string]interface{}) error {
	return nil
}

// ResetZeroPosition trivially implements motor.Motor.
func (tm *testMotor) ResetZeroPosition(_ context.Context, _ float64, _ map[string]interface{}) error {
	return nil
}

// Position trivially implements motor.Motor.
func (tm *testMotor) Position(_ context.Context, _ map[string]interface{}) (float64, error) {
	return 0.0, nil
}

// Properties trivially implements motor.Motor.
func (tm *testMotor) Properties(_ context.Context, _ map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{}, nil
}

// Stop trivially implements motor.Motor.
func (tm *testMotor) Stop(_ context.Context, _ map[string]interface{}) error {
	return nil
}

// IsPowered trivally implements motor.Motor.
func (tm *testMotor) IsPowered(_ context.Context, _ map[string]interface{}) (bool, float64, error) {
	return false, 0.0, nil
}

// DoCommand trivially implements motor.Motor.
func (tm *testMotor) DoCommand(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	//nolint:nilnil
	return nil, nil
}

// IsMoving trivially implements motor.Motor.
func (tm *testMotor) IsMoving(context.Context) (bool, error) {
	return false, nil
}
