// Package robot defines the robot which is the root of all robotic parts.
package robot_test

import (
	"testing"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gantry"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"go.viam.com/utils"
)

var (
	button1 = resource.NewName(resource.ResourceNamespaceRDK, resource.ResourceTypeComponent, resource.SubtypeName("button"), "arm1")

	armNames    = []resource.Name{arm.Named("arm1"), arm.Named("arm2")}
	buttonNames = []resource.Name{button1}
	sensorNames = []resource.Name{sensor.Named("sensor1")}
)

func setupInjectRobot() *inject.Robot {
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		if name.Name == "arm2" {
			return nil, false
		}
		return "here", true
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
	test.That(t, resources, test.ShouldResemble, []interface{}{"here", "here"})

	test.That(t, func() { robot.AllResourcesByName(r, "arm2") }, test.ShouldPanic)

	resources = robot.AllResourcesByName(r, "sensor1")
	test.That(t, resources, test.ShouldResemble, []interface{}{"here"})

	resources = robot.AllResourcesByName(r, "blah")
	test.That(t, resources, test.ShouldBeEmpty)
}

func TestNamesFromRobot(t *testing.T) {
	r := setupInjectRobot()

	names := robot.NamesBySubtype(r, gantry.Subtype)
	test.That(t, names, test.ShouldBeEmpty)

	names = robot.NamesBySubtype(r, sensor.Subtype)
	test.That(t, utils.NewStringSet(names...), test.ShouldResemble, utils.NewStringSet(testutils.ExtractNames(sensorNames...)...))

	names = robot.NamesBySubtype(r, arm.Subtype)
	test.That(t, utils.NewStringSet(names...), test.ShouldResemble, utils.NewStringSet(testutils.ExtractNames(armNames...)...))

}
