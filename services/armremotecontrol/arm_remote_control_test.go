package armremotecontrol

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	fakearm "go.viam.com/rdk/component/arm/fake"
	"go.viam.com/rdk/component/input"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

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
		return nil, rutils.NewResourceNotFoundError(name)
	}

	fakeController.RegisterControlCallbackFunc = func(
		ctx context.Context,
		control input.Control,
		triggers []input.EventType,
		ctrlFunc input.ControlFunction,
	) error {
		return nil
	}

	cfg := &Config{
		ArmName:               "",
		InputControllerName:   "",
		DefaultJointStep:      10.0,
		DefaultPoseStep:       0.10,
		ControllerSensitivity: 5.0,
		ControllerModes: []controllerMode{
			{
				ModeName: "joint",
				Mappings: map[input.Control]armPart{
					input.AbsoluteX:     jointOne,
					input.AbsoluteY:     jointTwo,
					input.AbsoluteRY:    jointThree,
					input.AbsoluteRX:    jointFour,
					input.AbsoluteHat0X: jointFive,
					input.AbsoluteHat0Y: jointSix,
				},
			}, {
				ModeName: "endpoint",
				Mappings: map[input.Control]armPart{
					input.AbsoluteX:     "ox",
					input.AbsoluteY:     "z",
					input.AbsoluteHat0X: "oz",
					input.AbsoluteHat0Y: "oy",
					input.AbsoluteRY:    "x",
					input.AbsoluteRX:    "y",
				},
			},
		},
	}

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
		return nil, rutils.NewResourceNotFoundError(name)
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
		return nil, rutils.NewResourceNotFoundError(name)
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
	t.Run("controller events supported", func(t *testing.T) {
		i := svc.controllerInputs()
		test.That(t, i[0], test.ShouldEqual, input.AbsoluteX)
		test.That(t, i[1], test.ShouldEqual, input.AbsoluteY)
		test.That(t, i[2], test.ShouldEqual, input.AbsoluteZ)
		test.That(t, i[3], test.ShouldEqual, input.AbsoluteRX)
		test.That(t, i[4], test.ShouldEqual, input.AbsoluteRY)
		test.That(t, i[5], test.ShouldEqual, input.AbsoluteRZ)
		test.That(t, i[6], test.ShouldEqual, input.AbsoluteHat0X)
		test.That(t, i[7], test.ShouldEqual, input.AbsoluteHat0Y)
		test.That(t, i[8], test.ShouldEqual, input.ButtonSouth)
		test.That(t, i[9], test.ShouldEqual, input.ButtonEast)
		test.That(t, i[10], test.ShouldEqual, input.ButtonWest)
		test.That(t, i[11], test.ShouldEqual, input.ButtonNorth)
		test.That(t, i[12], test.ShouldEqual, input.ButtonLT)
		test.That(t, i[13], test.ShouldEqual, input.ButtonRT)
		test.That(t, i[14], test.ShouldEqual, input.ButtonSelect)
		test.That(t, i[15], test.ShouldEqual, input.ButtonStart)
		test.That(t, i[16], test.ShouldEqual, input.ButtonMenu)
	})

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
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(&armRemoteService{}, nil))

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
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(&reconfigurableArmRemoteControl{}, nil))
}

func returnMock(name string) armRemoteService {
	return armRemoteService{
		config: &Config{ArmName: name},
	}
}

func stateShouldBeZero(state *controllerState) bool {
	for _, v := range state.buttons {
		if v {
			return false
		}
	}

	for _, v := range state.endpoints {
		if v > 0.0 {
			return false
		}
	}

	for _, v := range state.joints {
		if v > 0.0 {
			return false
		}
	}
	return true
}

func TestState(t *testing.T) {
	cfg := &Config{
		ArmName:               "",
		InputControllerName:   "",
		DefaultJointStep:      10.0,
		DefaultPoseStep:       0.10,
		ControllerSensitivity: 5.0,
		ControllerModes: []controllerMode{
			{
				ModeName: "joint",
				Mappings: map[input.Control]armPart{
					input.AbsoluteX:     jointOne,
					input.AbsoluteY:     jointTwo,
					input.AbsoluteRY:    jointThree,
					input.AbsoluteRX:    jointFour,
					input.AbsoluteHat0X: jointFive,
					input.AbsoluteHat0Y: jointSix,
				},
			}, {
				ModeName: "endpoint",
				Mappings: map[input.Control]armPart{
					input.AbsoluteX:     "ox",
					input.AbsoluteY:     "z",
					input.AbsoluteHat0X: "oz",
					input.AbsoluteHat0Y: "oy",
					input.AbsoluteRY:    "x",
					input.AbsoluteRX:    "y",
				},
			},
		},
	}

	// setup state
	state := &controllerState{}
	state.init()

	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.curModeIdx, test.ShouldEqual, 0)
	// button pressed
	state.set(input.Event{Time: time.Now(), Event: input.ButtonPress, Control: input.ButtonNorth, Value: 1}, *cfg)
	test.That(t, state.buttons[input.ButtonNorth], test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, buttonPressed)
	state.reset()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, noop)
	// joint test valid value
	state.set(input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 1.0}, *cfg)
	test.That(t, state.event, test.ShouldEqual, jointEvent)
	test.That(t, state.isInvalid(cfg.ControllerSensitivity), test.ShouldBeFalse)
	test.That(t, state.joints[jointOne], test.ShouldEqual, 1.0)
	state.reset()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, noop)
	// joint test invalid value
	state.set(input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.9}, *cfg)
	test.That(t, state.event, test.ShouldEqual, jointEvent)
	test.That(t, state.isInvalid(cfg.ControllerSensitivity), test.ShouldBeTrue)
	test.That(t, state.joints[jointOne], test.ShouldEqual, 0.9)
	state.reset()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, noop)
	// switch mode (emulating commands as these are not finalized)
	state.curModeIdx = 1
	// end point Valid Value
	state.set(input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 1.0}, *cfg)
	test.That(t, state.event, test.ShouldEqual, endPointEvent)
	test.That(t, state.isInvalid(cfg.ControllerSensitivity), test.ShouldBeFalse)
	test.That(t, state.endpoints[ox], test.ShouldEqual, 1.0)
	state.reset()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, noop)
	// end point test invalid value
	state.set(input.Event{Time: time.Now(), Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.9}, *cfg)
	test.That(t, state.event, test.ShouldEqual, endPointEvent)
	test.That(t, state.isInvalid(cfg.ControllerSensitivity), test.ShouldBeTrue)
	test.That(t, state.endpoints[ox], test.ShouldEqual, 0.9)
	state.reset()
	test.That(t, stateShouldBeZero(state), test.ShouldBeTrue)
	test.That(t, state.event, test.ShouldEqual, noop)
}
