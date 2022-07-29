package baseremotecontrol

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	fakearm "go.viam.com/rdk/component/base/arm"
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
	svc, ok := tmpSvc.(*remoteService)
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

	err = svc1.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc2.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc3.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	err = svc4.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	// Close out check
	err = utils.TryClose(context.Background(), svc)
	test.That(t, err, test.ShouldBeNil)
}

func TestLowLevel(t *testing.T) {
	test.That(t, scaleThrottle(.01), test.ShouldAlmostEqual, 0, .001)
	test.That(t, scaleThrottle(-.01), test.ShouldAlmostEqual, 0, .001)

	test.That(t, scaleThrottle(.33), test.ShouldAlmostEqual, 0.4, .001)
	test.That(t, scaleThrottle(.81), test.ShouldAlmostEqual, 0.9, .001)
	test.That(t, scaleThrottle(1.0), test.ShouldAlmostEqual, 1.0, .001)

	test.That(t, scaleThrottle(-.81), test.ShouldAlmostEqual, -0.9, .001)
	test.That(t, scaleThrottle(-1.0), test.ShouldAlmostEqual, -1.0, .001)
}

func TestSimilar(t *testing.T) {
	test.That(t, similar(r3.Vector{}, r3.Vector{}, 1), test.ShouldBeTrue)
	test.That(t, similar(r3.Vector{X: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
	test.That(t, similar(r3.Vector{Y: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
	test.That(t, similar(r3.Vector{Z: 2}, r3.Vector{}, 1), test.ShouldBeFalse)
}

func TestParseEvent(t *testing.T) {
	state := throttleState{}

	l, a := parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteX, Value: .5})
	test.That(t, similar(state.linearThrottle, r3.Vector{}, .1), test.ShouldBeTrue)
	test.That(t, similar(state.angularThrottle, r3.Vector{}, .1), test.ShouldBeTrue)

	test.That(t, similar(l, r3.Vector{}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{Z: -.5}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteY, Value: .5})
	test.That(t, similar(l, r3.Vector{Z: -.5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteRX, Value: .5})
	test.That(t, similar(l, r3.Vector{X: .5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)

	l, a = parseEvent(droneControl, &state, input.Event{Control: input.AbsoluteRY, Value: .5})
	test.That(t, similar(l, r3.Vector{Y: -.5}, .1), test.ShouldBeTrue)
	test.That(t, similar(a, r3.Vector{}, .1), test.ShouldBeTrue)
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
	rBRC, ok := reconfSvc.(*reconfigurableBaseRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	_, err = WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(&remoteService{}, nil))

	reconfSvc2, err := WrapWithReconfigurable(reconfSvc)
	test.That(t, err, test.ShouldBeNil)
	rBRC2, ok := reconfSvc2.(*reconfigurableBaseRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	name1 := rBRC.actual.config.BaseName
	name2 := rBRC2.actual.config.BaseName

	test.That(t, name1, test.ShouldEqual, name2)
}

func TestReconfigure(t *testing.T) {
	actualSvc := returnMock("svc1")
	reconfSvc, err := WrapWithReconfigurable(actualSvc)
	test.That(t, err, test.ShouldBeNil)
	rBRC, ok := reconfSvc.(*reconfigurableBaseRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)

	actualSvc2 := returnMock("svc1")
	reconfSvc2, err := WrapWithReconfigurable(actualSvc2)
	test.That(t, err, test.ShouldBeNil)
	rBRC2, ok := reconfSvc2.(*reconfigurableBaseRemoteControl)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, rBRC2, test.ShouldNotBeNil)

	err = reconfSvc.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	name1 := rBRC.actual.config.BaseName
	name2 := rBRC2.actual.config.BaseName
	test.That(t, name1, test.ShouldEqual, name2)

	err = reconfSvc.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(&reconfigurableBaseRemoteControl{}, nil))
}

func returnMock(name string) remoteService {
	return remoteService{
		config: &Config{BaseName: name},
	}
}
