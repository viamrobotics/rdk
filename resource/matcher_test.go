package resource_test

import (
	"testing"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/testutils"
	"go.viam.com/test"
)

func TestMatchers(t *testing.T) {
	armComponent := testutils.NewUnimplementedResource(arm.Named("arm"))
	sensorService := testutils.NewUnimplementedResource(sensor.Named("sensor"))
	motionService := testutils.NewUnimplementedResource(motion.Named("motion"))
	t.Run("type matcher", func(t *testing.T) {
		matcher := resource.TypeMatcher{Type: resource.APITypeComponentName}
		test.That(t, matcher.IsMatch(armComponent), test.ShouldBeTrue)
		test.That(t, matcher.IsMatch(motionService), test.ShouldBeFalse)
	})

	t.Run("subtype matcher", func(t *testing.T) {
		matcher := resource.SubtypeMatcher{Subtype: sensor.SubtypeName}
		test.That(t, matcher.IsMatch(sensorService), test.ShouldBeTrue)
		test.That(t, matcher.IsMatch(motionService), test.ShouldBeFalse)
	})

	t.Run("interface matcher", func(t *testing.T) {
		// define a resource that trivially satisfies the Actuator interface
		type unimplActuator struct {
			resource.Resource
			resource.Actuator
		}
		var testActuator unimplActuator

		matcher := resource.InterfaceMatcher{Interface: new(resource.Actuator)}
		test.That(t, matcher.IsMatch(testActuator), test.ShouldBeTrue)
		test.That(t, matcher.IsMatch(armComponent), test.ShouldBeFalse)
	})
}
