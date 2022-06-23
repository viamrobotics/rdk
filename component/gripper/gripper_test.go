package gripper_test

import (
	"context"
	"testing"

	"github.com/mitchellh/mapstructure"
	"go.viam.com/test"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testGripperName    = "gripper1"
	testGripperName2   = "gripper2"
	failGripperName    = "gripper3"
	fakeGripperName    = "gripper4"
	missingGripperName = "gripper5"
)

func setupInjectRobot() *inject.Robot {
	gripper1 := &mock{Name: testGripperName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case gripper.Named(testGripperName):
			return gripper1, nil
		case gripper.Named(fakeGripperName):
			return "not a gripper", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{gripper.Named(testGripperName), arm.Named("arm1")}
	}
	return r
}

func TestGenericDo(t *testing.T) {
	r := setupInjectRobot()

	g, err := gripper.FromRobot(r, testGripperName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g, test.ShouldNotBeNil)

	command := map[string]interface{}{"cmd": "test", "data1": 500}
	ret, err := g.Do(context.Background(), command)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ret, test.ShouldEqual, command)
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := gripper.FromRobot(r, testGripperName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	result, err := res.Grab(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, grabbed)

	res, err = gripper.FromRobot(r, fakeGripperName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Gripper", "string"))
	test.That(t, res, test.ShouldBeNil)

	res, err = gripper.FromRobot(r, missingGripperName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(gripper.Named(missingGripperName)))
	test.That(t, res, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := gripper.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testGripperName})
}

func TestStatusValid(t *testing.T) {
	status := &commonpb.ActuatorStatus{
		IsMoving: true,
	}
	map1, err := protoutils.InterfaceToMap(status)
	test.That(t, err, test.ShouldBeNil)
	newStruct, err := structpb.NewStruct(map1)
	test.That(t, err, test.ShouldBeNil)
	test.That(
		t,
		newStruct.AsMap(),
		test.ShouldResemble,
		map[string]interface{}{
			"is_moving": true,
		},
	)

	convMap := &commonpb.ActuatorStatus{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &convMap})
	test.That(t, err, test.ShouldBeNil)
	err = decoder.Decode(newStruct.AsMap())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, convMap, test.ShouldResemble, status)
}

func TestCreateStatus(t *testing.T) {
	_, err := gripper.CreateStatus(context.Background(), "not a gripper")
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGripper", "string"))

	t.Run("is moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: true,
		}

		injectGripper := &inject.Gripper{}
		injectGripper.IsMovingFunc = func(context.Context) (bool, error) {
			return true, nil
		}
		status1, err := gripper.CreateStatus(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)

		resourceSubtype := registry.ResourceSubtypeLookup(gripper.Subtype)
		status2, err := resourceSubtype.Status(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status2, test.ShouldResemble, status)
	})

	t.Run("is not moving", func(t *testing.T) {
		status := &commonpb.ActuatorStatus{
			IsMoving: false,
		}

		injectGripper := &inject.Gripper{}
		injectGripper.IsMovingFunc = func(context.Context) (bool, error) {
			return false, nil
		}
		status1, err := gripper.CreateStatus(context.Background(), injectGripper)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, status1, test.ShouldResemble, status)
	})
}

func TestGripperName(t *testing.T) {
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
					ResourceSubtype: gripper.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testGripperName,
			resource.Name{
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: gripper.SubtypeName,
				},
				Name: testGripperName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := gripper.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualGripper1 gripper.Gripper = &mock{Name: testGripperName}
	reconfGripper1, err := gripper.WrapWithReconfigurable(actualGripper1)
	test.That(t, err, test.ShouldBeNil)

	_, err = gripper.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("LocalGripper", nil))

	reconfGripper2, err := gripper.WrapWithReconfigurable(reconfGripper1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGripper2, test.ShouldEqual, reconfGripper1)
}

func TestReconfigurableGripper(t *testing.T) {
	actualGripper1 := &mock{Name: testGripperName}
	reconfGripper1, err := gripper.WrapWithReconfigurable(actualGripper1)
	test.That(t, err, test.ShouldBeNil)

	actualGripper2 := &mock{Name: testGripperName}
	reconfGripper2, err := gripper.WrapWithReconfigurable(actualGripper2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualGripper1.reconfCount, test.ShouldEqual, 0)

	err = reconfGripper1.Reconfigure(context.Background(), reconfGripper2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfGripper1, test.ShouldResemble, reconfGripper2)
	test.That(t, actualGripper1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualGripper1.grabCount, test.ShouldEqual, 0)
	test.That(t, actualGripper2.grabCount, test.ShouldEqual, 0)
	result, err := reconfGripper1.(gripper.Gripper).Grab(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, grabbed)
	test.That(t, actualGripper1.grabCount, test.ShouldEqual, 0)
	test.That(t, actualGripper2.grabCount, test.ShouldEqual, 1)

	err = reconfGripper1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *gripper.reconfigurableGripper")
}

func TestStop(t *testing.T) {
	actualGripper1 := &mock{Name: testGripperName}
	reconfGripper1, err := gripper.WrapWithReconfigurable(actualGripper1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualGripper1.stopCount, test.ShouldEqual, 0)
	test.That(t, reconfGripper1.(gripper.Gripper).Stop(context.Background()), test.ShouldBeNil)
	test.That(t, actualGripper1.stopCount, test.ShouldEqual, 1)
}

const grabbed = true

type mock struct {
	gripper.LocalGripper
	Name        string
	grabCount   int
	stopCount   int
	reconfCount int
}

func (m *mock) Grab(ctx context.Context) (bool, error) {
	m.grabCount++
	return grabbed, nil
}

func (m *mock) Stop(ctx context.Context) error {
	m.stopCount++
	return nil
}

func (m *mock) Close() { m.reconfCount++ }

func (m *mock) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
