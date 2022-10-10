package builtin

import (
	"context"
	"strconv"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/arm"
	fakearm "go.viam.com/rdk/components/arm/fake"
	"go.viam.com/rdk/components/arm/xarm"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func buildCfg(dof int) *ServiceConfig {
	cfg := &ServiceConfig{
		ArmName:               "",
		InputControllerName:   "",
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

	fakeRobot := &inject.Robot{}
	fakeController := &inject.InputController{}

	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name.Subtype {
		case input.Subtype:
			return fakeController, nil
		case arm.Subtype:
			return fakearm.NewArm(
				config.Component{
					Name:                arm.Subtype.String(),
					ConvertedAttributes: &fakearm.AttrConfig{ArmModel: xarm.ModelName6DOF},
				},
				logger,
			)
		}
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

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
	tmpSvc, err := NewBuiltIn(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeNil)

	svc, ok := tmpSvc.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	// Controller import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == arm.Subtype {
			return &fakearm.Arm{}, nil
		}
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	_, err = NewBuiltIn(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:input_controller/\" not found"))

	// Arm import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == input.Subtype {
			return fakeController, nil
		}
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	_, err = NewBuiltIn(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:arm/\" not found"))

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
