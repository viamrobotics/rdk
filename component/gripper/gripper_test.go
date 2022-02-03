package gripper_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
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
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		switch name {
		case gripper.Named(testGripperName):
			return gripper1, true
		case gripper.Named(fakeGripperName):
			return "not a gripper", false
		default:
			return nil, false
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{gripper.Named(testGripperName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, ok := gripper.FromRobot(r, testGripperName)
	test.That(t, res, test.ShouldNotBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	result, err := res.Grab(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, grabbed)

	res, ok = gripper.FromRobot(r, fakeGripperName)
	test.That(t, res, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)

	res, ok = gripper.FromRobot(r, missingGripperName)
	test.That(t, res, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := gripper.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testGripperName})
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
				UUID: "e2b52bce-800b-56b7-904c-2f8372ce4623",
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
				UUID: "f3e34221-62ec-5951-b112-d4cccb47bf61",
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
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected resource")

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

const grabbed = true

type mock struct {
	gripper.Gripper
	Name        string
	grabCount   int
	reconfCount int
}

func (m *mock) Grab(ctx context.Context) (bool, error) {
	m.grabCount++
	return grabbed, nil
}

func (m *mock) Close() { m.reconfCount++ }
