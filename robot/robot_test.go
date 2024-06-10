package robot_test

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/gantry"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
)

var (
	button1 = resource.NewName(resource.APINamespaceRDK.WithComponentType("button"), "arm1")

	armNames    = []resource.Name{arm.Named("arm1"), arm.Named("arm2"), arm.Named("remote:arm1")}
	buttonNames = []resource.Name{button1}
	sensorNames = []resource.Name{sensor.Named("sensor1")}
)

var hereRes = testutils.NewUnimplementedResource(generic.Named("here"))

func setupInjectRobot() *inject.Robot {
	arm3 := inject.NewArm("arm3")
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		if name.Name == "arm2" {
			return nil, resource.NewNotFoundError(name)
		}
		if name.Name == "arm3" {
			return arm3, nil
		}

		return hereRes, nil
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return testutils.ConcatResourceNames(
			armNames,
			buttonNames,
			sensorNames,
		)
	}

	return r
}

func TestAllResourcesByName(t *testing.T) {
	r := setupInjectRobot()

	resources := robot.AllResourcesByName(r, "arm1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes, hereRes})

	resources = robot.AllResourcesByName(r, "remote:arm1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes})

	test.That(t, func() { robot.AllResourcesByName(r, "arm2") }, test.ShouldPanic)

	resources = robot.AllResourcesByName(r, "sensor1")
	test.That(t, resources, test.ShouldResemble, []resource.Resource{hereRes})

	resources = robot.AllResourcesByName(r, "blah")
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := robot.NamesByAPI(r, gantry.API)
	test.That(t, names, test.ShouldBeEmpty)

	names = robot.NamesByAPI(r, sensor.API)
	testutils.VerifySameElements(t, names, testutils.ExtractNames(sensorNames...))

	names = robot.NamesByAPI(r, arm.API)
	testutils.VerifySameElements(t, names, testutils.ExtractNames(armNames...))
}

func TestResourceFromRobot(t *testing.T) {
	r := setupInjectRobot()

	res, err := robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm3"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, res, test.ShouldNotBeNil)

	res, err = robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm5"))
	test.That(t, err, test.ShouldBeError,
		resource.TypeError[arm.Arm](testutils.NewUnimplementedResource(generic.Named("foo"))))
	test.That(t, res, test.ShouldBeNil)

	res, err = robot.ResourceFromRobot[arm.Arm](r, arm.Named("arm2"))
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(arm.Named("arm2")))
	test.That(t, res, test.ShouldBeNil)
}

func TestMatchesModule(t *testing.T) {
	idRequest := robot.RestartModuleRequest{ModuleID: "matching-id"}
	test.That(t, idRequest.MatchesModule(config.Module{ModuleID: "matching-id"}), test.ShouldBeTrue)
	test.That(t, idRequest.MatchesModule(config.Module{Name: "matching-id"}), test.ShouldBeFalse)
	test.That(t, idRequest.MatchesModule(config.Module{ModuleID: "other"}), test.ShouldBeFalse)

	nameRequest := robot.RestartModuleRequest{ModuleName: "matching-name"}
	test.That(t, nameRequest.MatchesModule(config.Module{Name: "matching-name"}), test.ShouldBeTrue)
	test.That(t, nameRequest.MatchesModule(config.Module{ModuleID: "matching-name"}), test.ShouldBeFalse)
	test.That(t, nameRequest.MatchesModule(config.Module{Name: "other"}), test.ShouldBeFalse)
}
