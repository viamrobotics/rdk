package input_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/input"
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

func setupInjectRobot() *inject.Robot {
	inputController1 := &mock{Name: testInputControllerName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case input.Named(testInputControllerName):
			return inputController1, true
		case input.Named(fakeInputControllerName):
			return "not an input controller", true
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{input.Named(testInputControllerName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := input.FromRobot(r, testInputControllerName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.GetControls(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldResemble, controls)

	res, err = input.FromRobot(r, fakeInputControllerName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("input.Controller", "string"))
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
				UUID: "48d8bd5e-629b-51c1-8bf8-f2f308942012",
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
				UUID: "6a00798b-dab4-5abc-b3d9-7fc45f9cfea0",
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
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1)
	test.That(t, err, test.ShouldBeNil)

	_, err = input.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("input.Controller", nil))

	reconfInput2, err := input.WrapWithReconfigurable(reconfInput1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfInput2, test.ShouldEqual, reconfInput1)
}

func TestReconfigurableInputController(t *testing.T) {
	actualInput1 := &mock{Name: testInputControllerName}
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1)
	test.That(t, err, test.ShouldBeNil)

	actualInput2 := &mock{Name: testInputControllerName2}
	reconfInput2, err := input.WrapWithReconfigurable(actualInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 0)

	err = reconfInput1.Reconfigure(context.Background(), reconfInput2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfInput1, test.ShouldResemble, reconfInput2)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualInput1.controlsCount, test.ShouldEqual, 0)
	test.That(t, actualInput2.controlsCount, test.ShouldEqual, 0)
	result, err := reconfInput1.(input.Controller).GetControls(context.Background())
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
	reconfInput1, err := input.WrapWithReconfigurable(actualInput1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfInput1), test.ShouldBeNil)
	test.That(t, actualInput1.reconfCount, test.ShouldEqual, 1)
}

var controls = []input.Control{input.AbsoluteX}

type mock struct {
	input.Controller
	Name          string
	controlsCount int
	reconfCount   int
}

func (m *mock) GetControls(ctx context.Context) ([]input.Control, error) {
	m.controlsCount++
	return controls, nil
}

func (m *mock) Close() { m.reconfCount++ }
