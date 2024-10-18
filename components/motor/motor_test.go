package motor_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testMotorName = "motor1"
	failMotorName = "motor2"
	fakeMotorName = "motor3"
)

func TestFromRobot(t *testing.T) {
	r := &inject.Robot{}
	rs := map[resource.Name]resource.Resource{
		motor.Named("m1"):  inject.NewMotor("m1"),
		motor.Named("m2"):  inject.NewMotor("m2"),
		generic.Named("g"): inject.NewGenericComponent("g"),
	}
	r.MockResourcesFromMap(rs)

	expected := []string{"m1", "m2"}
	testutils.VerifySameElements(t, motor.NamesFromRobot(r), expected)

	_, err := motor.FromRobot(r, "m1")
	test.That(t, err, test.ShouldBeNil)

	_, err = motor.FromRobot(r, "m2")
	test.That(t, err, test.ShouldBeNil)

	_, err = motor.FromRobot(r, "m0")
	test.That(t, err, test.ShouldNotBeNil)

	_, err = motor.FromRobot(r, "g")
	test.That(t, err, test.ShouldNotBeNil)
}
