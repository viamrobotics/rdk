package servo_test

import (
	"context"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/servo"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testServoName    = "servo1"
	testServoName2   = "servo2"
	failServoName    = "servo3"
	fakeServoName    = "servo4"
	missingServoName = "servo5"
)

func setupInjectRobot() *inject.Robot {
	servo1 := &mock{Name: testServoName}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		switch name {
		case servo.Named(testServoName):
			return servo1, nil
		case servo.Named(fakeServoName):
			return "not a servo", nil
		default:
			return nil, rutils.NewResourceNotFoundError(name)
		}
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{servo.Named(testServoName), arm.Named("arm1")}
	}
	return r
}

func TestFromRobot(t *testing.T) {
	r := setupInjectRobot()

	s, err := servo.FromRobot(r, testServoName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, s, test.ShouldNotBeNil)

	result, err := s.GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, pos)

	s, err = servo.FromRobot(r, fakeServoName)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Servo", "string"))
	test.That(t, s, test.ShouldBeNil)

	s, err = servo.FromRobot(r, missingServoName)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(servo.Named(missingServoName)))
	test.That(t, s, test.ShouldBeNil)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := servo.NamesFromRobot(r)
	test.That(t, names, test.ShouldResemble, []string{testServoName})
}

func TestServoName(t *testing.T) {
	for _, tc := range []struct {
		TestName string
		Name     string
		Expected resource.Name
	}{
		{
			"missing name",
			"",
			resource.Name{
				UUID: "90cdc3ec-bf17-568f-8340-c6add982e00f",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: servo.SubtypeName,
				},
				Name: "",
			},
		},
		{
			"all fields included",
			testServoName,
			resource.Name{
				UUID: "85bbeb08-07b7-5fef-8706-27258bc67859",
				Subtype: resource.Subtype{
					Type:            resource.Type{Namespace: resource.ResourceNamespaceRDK, ResourceType: resource.ResourceTypeComponent},
					ResourceSubtype: servo.SubtypeName,
				},
				Name: testServoName,
			},
		},
	} {
		t.Run(tc.TestName, func(t *testing.T) {
			observed := servo.Named(tc.Name)
			test.That(t, observed, test.ShouldResemble, tc.Expected)
		})
	}
}

func TestWrapWithReconfigurable(t *testing.T) {
	var actualServo1 servo.Servo = &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	_, err = servo.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("Servo", nil))

	reconfServo2, err := servo.WrapWithReconfigurable(reconfServo1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfServo2, test.ShouldEqual, reconfServo1)
}

func TestReconfigurableServo(t *testing.T) {
	actualServo1 := &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	actualServo2 := &mock{Name: testServoName2}
	reconfServo2, err := servo.WrapWithReconfigurable(actualServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 0)

	err = reconfServo1.Reconfigure(context.Background(), reconfServo2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfServo1, test.ShouldResemble, reconfServo2)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 1)

	test.That(t, actualServo1.posCount, test.ShouldEqual, 0)
	test.That(t, actualServo2.posCount, test.ShouldEqual, 0)
	result, err := reconfServo1.(servo.Servo).GetPosition(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, pos)
	test.That(t, actualServo1.posCount, test.ShouldEqual, 0)
	test.That(t, actualServo2.posCount, test.ShouldEqual, 1)

	err = reconfServo1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected *servo.reconfigurableServo")
}

func TestClose(t *testing.T) {
	actualServo1 := &mock{Name: testServoName}
	reconfServo1, err := servo.WrapWithReconfigurable(actualServo1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 0)
	test.That(t, utils.TryClose(context.Background(), reconfServo1), test.ShouldBeNil)
	test.That(t, actualServo1.reconfCount, test.ShouldEqual, 1)
}

const pos = 3

type mock struct {
	Name string

	posCount    int
	reconfCount int
}

func (mServo *mock) Move(ctx context.Context, angleDegs uint8) error {
	return nil
}

func (mServo *mock) GetPosition(ctx context.Context) (uint8, error) {
	mServo.posCount++
	return pos, nil
}

func (mServo *mock) Close() { mServo.reconfCount++ }
