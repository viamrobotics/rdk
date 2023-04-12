package input_test

import (
	"context"
	"testing"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/inputcontroller/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/input"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testInputControllerName    = "inputController1"
	testInputControllerName2   = "inputController2"
	failInputControllerName    = "inputController3"
	fakeInputControllerName    = "inputController4"
	missingInputControllerName = "inputController5"
)

func setupDependencies(t *testing.T) registry.Dependencies {
	t.Helper()

	deps := make(registry.Dependencies)
	deps[input.Named(testInputControllerName)] = &mock{Name: testInputControllerName}
	deps[input.Named(fakeInputControllerName)] = "not an input controller"
	return deps
}

func setupInjectRobot() *inject.Robot {
	inputController1 := &mock{Name: testInputControllerName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case input.Named(testInputControllerName):
			return inputController1, nil
		case input.Named(fakeInputControllerName):
			return "not an input controller", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{input.Named(testInputControllerName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	i, err := input.FromRobot(r, testInputControllerName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, i, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := i.DoCommand(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromDependencies(t *testing.T) {
	deps := setupDependencies(t)

	res, err := input.FromDependencies(deps, testInputControllerName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.Controls(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, controls)

	res, err = input.FromDependencies(deps, fakeInputControllerName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyTypeError[input.Controller](fakeInputControllerName, "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = input.FromDependencies(deps, missingInputControllerName)
	test.That(t, err, test.ShouldBeError, rutils.DependencyNotFoundError(missingInputControllerName))
	test.That(t, res, test.ShouldBeNil)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := input.FromRobot(r, testInputControllerName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.Controls(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, controls)

	res, err = input.FromRobot(r, fakeInputControllerName)
	test.That(t, err, test.ShouldBeError, input.NewUnimplementedInterfaceError("string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = input.FromRobot(r, missingInputControllerName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(input.Named(missingInputControllerName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := input.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testInputControllerName})
}

func TestStatusValid(t *testing.T) {
	timestamp := timestamppb.Now()
	status := &pb.Status{
		Events: []*pb.Event{{Time: timestamp, Event: string(input.PositionChangeAbs), Control: string(input.AbsoluteX), Value: 0.7}},
	}
	newStruct, err := protoutils.StructToStructPb(status)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"events": []interface{}{
				map[string]interface{}{
					"control": "AbsoluteX",
					"event":   "PositionChangeAbs",
					"time":    map[string]interface{}{"nanos": float64(timestamp.Nanos), "seconds": float64(timestamp.Seconds)},
					"value":   0.7,
				},
			},
		},
	)

	convMap := &pb.Status{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	_, err := input.CreateStatus(context.Background(), "not an input")
	test.That(t, err, test.ShouldBeError, input.NewUnimplementedInterfaceError("string"))

	timestamp := time.Now()
	event := input.Event{Time: timestamp, Event: input.PositionChangeAbs, Control: input.AbsoluteX, Value: 0.7}
	status := &pb.Status{
		Events: []*pb.Event{
			{Time: timestamppb.New(timestamp), Event: string(event.Event), Control: string(event.Control), Value: event.Value},
		},
	}
	injectInputController := &inject.InputController{}
	injectInputController.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
		eventsOut := make(map[input.Control]input.Event)
		eventsOut[input.AbsoluteX] = event
		return eventsOut, nil
	}

	t.Run("working", func(t *testing.T) {
		status1, err := input.CreateStatus(context.Background(), injectInputController)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})

	t.Run("fail on Events", func(t *testing.T) {
		errFail := errors.New("can't get events")
		injectInputController.EventsFunc = func(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
			return nil, errFail
		}
		_, err = input.CreateStatus(context.Background(), injectInputController)
		test.That(t, err, test.ShouldBeError, errFail)
	})
}

func TestInputControllerName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: input.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testInputControllerName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: input.SubtypeName,
				},
				Name: testInputControllerName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := input.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualInput1 input.Controller = &mock{Name: testInputControllerName}
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	_, err = input.WrapWithReconfigurable(nil, resource.Name{})
	test.That(t, err, test.ShouldBeError, input.NewUnimplementedInterfaceError(nil))

	reconfInput2, err := input.WrapWithReconfigurable(reconfInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfInput2, test.ShouldEqual, reconfInput1)
}

func TestReconfigurableInputController(t *testing.T) {
	actualInput1 := &mock{Name: testInputControllerName}
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	actualInput2 := &mock{Name: testInputControllerName2}
	reconfInput2, err := input.WrapWithReconfigurable(actualInput2, resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 0)

	err = reconfInput1.Reconfigure(context.Background(), reconfInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfInput1, test.ShouldResemble, reconfInput2)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualInput1.controlsCount, test.ShouldEqual, 0)
	test.That(t, actualInput2.controlsCount, test.ShouldEqual, 0)
	result, err := reconfInput1.(input.Controller).Controls(context.Background(), map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, controls)
	test.That(t, actualInput1.controlsCount, test.ShouldEqual, 0)
	test.That(t, actualInput2.controlsCount, test.ShouldEqual, 1)

	err = reconfInput1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *input.reconfigurableInputController")
}

func TestClose(t *testing.T) {
	actualInput1 := &mock{Name: testInputControllerName}
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1, resource.Name{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfInput1), test.ShouldBeNil)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)
}

func TestExtra(t *testing.T) {
	actualInput1 := &mock{Name: testInputControllerName}
	reconfInput1, err := input.WrapWithReconfigurable((actualInput1), resource.Name{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualInput1.extra, test.ShouldBeNil)

	extra := map[string]interface{}{"foo": "Controls"}
	reconfInput1.(input.Controller).Controls(context.Background(), extra)
	test.That(t, actualInput1.extra, test.ShouldResemble, extra)

	extra = map[string]interface{}{"foo": "Events"}
	reconfInput1.(input.Controller).Events(context.Background(), extra)
	test.That(t, actualInput1.extra, test.ShouldResemble, extra)

	extra = map[string]interface{}{"foo": "RegisterControlCallback"}
	reconfInput1.(input.Controller).RegisterControlCallback(context.Background(), input.AbsoluteHat0X, nil, nil, extra)
	test.That(t, actualInput1.extra, test.ShouldResemble, extra)
}

var controls = []input.Control{input.AbsoluteX}

type mock struct {
	input.Controller
	Name          string
	controlsCount int
	reconfCount   int

	extra map[string]interface{}
}

func (m *mock) Controls(ctx context.Context, extra map[string]interface{}) ([]input.Control, error) {
	m.controlsCount++
	m.extra = extra
	return controls, nil
}

func (m *mock) Events(ctx context.Context, extra map[string]interface{}) (map[input.Control]input.Event, error) {
	m.extra = extra
	return make(map[input.Control]input.Event), nil
}

func (m *mock) RegisterControlCallback(
	ctx context.Context,
	control input.Control,
	triggers []input.EventType,
	ctrlFunc input.ControlFunction,
	extra map[string]interface{},
) error {
	m.extra = extra
	return nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
