package builtin

import (
	"context"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	fakearm "go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/arm/xarm"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/testutils/inject"
)

func buildCfg(dof int) *ServiceConfig {
	cfg := &ServiceConfig{
		ArmName:               "armTest",
		InputControllerName:   "controllerTest",
		JointStep:             10.0,
		MMStep:                0.1,
		DegreeStep:            5.0,
		ControllerSensitivity: 5.0,
		ControllerModes: []ControllerMode{
			{
				ModeName:       jointMode,
				ControlMapping: map[string]input.Control{},
			},
			{
				ModeName: endpointMode,
				ControlMapping: map[string]input.Control{
					"x":     input.AbsoluteX,
					"y":     input.AbsoluteY,
					"z":     input.AbsoluteHat0X,
					"roll":  input.AbsoluteHat0Y,
					"pitch": input.AbsoluteRY,
					"yaw":   input.AbsoluteRX,
				},
			},
		},
	}
	for i := 0; i < dof; i++ {
		idx := strconv.Itoa(i)
		cfg.ControllerModes[0].ControlMapping[idx] = input.AbsoluteX
	}
	return cfg
}

func TestArmRemoteControl(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg := buildCfg(6)
	deps := make(registry.Dependencies)

	fakeController := &inject.InputController{}
	fakeArm, _ := fakearm.NewArm(
		config.Component{
			Name:                arm.Subtype.String(),
			ConvertedAttributes: &fakearm.AttrConfig{ArmModel: xarm.ModelName6DOF},
		},
		logger,
	)

	deps[arm.Named(cfg.ArmName)] = fakeArm
	deps[input.Named(cfg.InputControllerName)] = fakeController

	fakeController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		return nil
	}

	fakeController.ControlsFunc = func(ctx context.Context) ([]input.Control, error) {
		r := make([]input.Control, 1)
		r[0] = input.ButtonMenu
		return r, nil
	}

	// New arm_remote_control check
	tmpSvc, err := NewBuiltIn(ctx, deps,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)

	svc, ok := tmpSvc.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	delete(deps, input.Named(cfg.InputControllerName))
	deps[arm.Named(cfg.ArmName)] = &fakearm.Arm{}

	_, err = NewBuiltIn(ctx, deps,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("\"controllerTest\" missing from dependencies"))

	deps[input.Named(cfg.InputControllerName)] = fakeController
	delete(deps, arm.Named(cfg.ArmName))

	_, err = NewBuiltIn(ctx, deps,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("\"armTest\" missing from dependencies"))

	// Deps exist but are incorrect component
	deps[arm.Named(cfg.ArmName)] = fakeController
	deps[input.Named(cfg.InputControllerName)] = fakeController
	_, err = NewBuiltIn(ctx, deps,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError,
		errors.New("dependency \"armTest\" should an implementation of <nil> but it was a *inject.InputController"))

	// Start checks
	err = svc.start(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Controller event supported
	err = svc.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Close out check
	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
}

func stateShouldBeZero(state *controllerState) bool {
	for _, v := range state.buttons {
		if v {
			return false
		}
	}
	return true
}

func TestState(t *testing.T) {
	state := &controllerState{}
	state.init()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
}
