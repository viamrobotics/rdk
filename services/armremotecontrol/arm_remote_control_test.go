package armremotecontrol

import (
	"context"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"go.viam.com/test"
	utils "go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	fakearm "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func buildCfg(dof int) *ServiceConfig {
	cfg := &ServiceConfig{
		ArmName: "",
		InputControllerName: "",
		JointStep: 10.0,
		MMStep: 0.1,
		DegreeStep: 5.0,
		ControllerSensitivity: 5.0,
		ControllerModes: []ControllerMode{
			{
				ModeName: jointMode,
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

	fakeRobot := &inject.Robot{}
	fakeController := &inject.InputController{}

	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name.Subtype {
		case input.Subtype:
			return fakeController, nil
		case arm.Subtype:
			return &fakearm.Arm{}, nil
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

	cfg := buildCfg(6)

	// New arm_remote_control check
	tmpSvc, err := New(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeNil)
	svc, ok := tmpSvc.(*armRemoteService)
	test.That(t, ok, test.ShouldBeTrue)

	// Controller import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == arm.Subtype {
			return &fakearm.Arm{}, nil
		}
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:input_controller\" not found"))

	// Arm import failure
	fakeRobot.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		if name.Subtype == input.Subtype {
			return fakeController, nil
		}
		return nil, rdkutils.NewResourceNotFoundError(name)
	}

	_, err = New(ctx, fakeRobot,
		config.Service{
			Name:                "arm_remote_control",
			Type:                "arm_remote_control",
			ConvertedAttributes: cfg,
		},
		rlog.Logger)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:arm\" not found"))

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

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	actualSvc := returnMock("svc1")
	reconfSvc, err := WrapWithReconfigurable(actualSvc)
	test.That(t, err, test.ShouldBeNil)
	rBRC, ok := reconfSvc.(*reconfigurableArmRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rdkutils.NewUnexpectedTypeError(&armRemoteService{}, nil))

	reconfSvc2, err := WrapWithReconfigurable(reconfSvc)
	test.That(t, err, test.ShouldBeNil)
	rBRC2, ok := reconfSvc2.(*reconfigurableArmRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	name1 := rBRC.actual.config.ArmName
	name2 := rBRC2.actual.config.ArmName

	test.That(t, name1, test.ShouldEqual, name2)
}

func TestReconfigure(t *testing.T) {
	actualSvc := returnMock("svc1")
	reconfSvc, err := WrapWithReconfigurable(actualSvc)
	test.That(t, err, test.ShouldBeNil)
	rBRC, ok := reconfSvc.(*reconfigurableArmRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	actualSvc2 := returnMock("svc1")
	reconfSvc2, err := WrapWithReconfigurable(actualSvc2)
	test.That(t, err, test.ShouldBeNil)
	rBRC2, ok := reconfSvc2.(*reconfigurableArmRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rBRC2, test.ShouldNotBeNil)

	err = reconfSvc.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	name1 := rBRC.actual.config.ArmName
	name2 := rBRC2.actual.config.ArmName
	test.That(t, name1, test.ShouldEqual, name2)

	err = reconfSvc.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldBeError, rdkutils.NewUnexpectedTypeError(&reconfigurableArmRemoteControl{}, nil))
}

func returnMock(name string) armRemoteService {
	return armRemoteService{
		config: &ServiceConfig{ArmName: name},
	}
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
