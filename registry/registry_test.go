package registry

import (
	"context"
	"testing"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	pf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (robot.Provider, error) {
		return nil, nil
	}

	af := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return nil, nil
	}

	cf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error) {
		return nil, nil
	}

	gf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error) {
		return nil, nil
	}

	lf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error) {
		return nil, nil
	}

	sf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error) {
		return nil, nil
	}

	bf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error) {
		return nil, nil
	}
	ff := func(name string) (referenceframe.Frame, error) {
		return nil, nil
	}

	// test panics
	test.That(t, func() { RegisterProvider("x", ProviderRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterCamera("x", CameraRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterArm("x", ArmRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterBase("x", BaseRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterGripper("x", GripperRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterLidar("x", LidarRegistration{}) }, test.ShouldPanic)
	test.That(t, func() { RegisterSensor(sensor.Type("x"), "y", SensorRegistration{}) }, test.ShouldPanic)

	RegisterProvider("x", ProviderRegistration{Constructor: pf})
	RegisterCamera("x", CameraRegistration{cf, ff})
	RegisterBase("x", BaseRegistration{Constructor: bf, Frame: ff})
	RegisterArm("x", ArmRegistration{Constructor: af, Frame: ff})
	RegisterGripper("x", GripperRegistration{gf, ff})
	RegisterLidar("x", LidarRegistration{Constructor: lf})
	RegisterSensor(sensor.Type("x"), "y", SensorRegistration{Constructor: sf, Frame: ff})

	test.That(t, ProviderLookup("x"), test.ShouldNotBeNil)
	test.That(t, ProviderLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, ProviderLookup("x").Frame, test.ShouldBeNil)
	test.That(t, CameraLookup("x"), test.ShouldNotBeNil)
	test.That(t, CameraLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, CameraLookup("x").Frame, test.ShouldNotBeNil)
	test.That(t, ArmLookup("x"), test.ShouldNotBeNil)
	test.That(t, ArmLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, ArmLookup("x").Frame, test.ShouldNotBeNil)
	test.That(t, BaseLookup("x"), test.ShouldNotBeNil)
	test.That(t, BaseLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, BaseLookup("x").Frame, test.ShouldNotBeNil)
	test.That(t, GripperLookup("x"), test.ShouldNotBeNil)
	test.That(t, GripperLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, GripperLookup("x").Frame, test.ShouldNotBeNil)
	test.That(t, LidarLookup("x"), test.ShouldNotBeNil)
	test.That(t, LidarLookup("x").Constructor, test.ShouldNotBeNil)
	test.That(t, LidarLookup("x").Frame, test.ShouldBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Constructor, test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y").Frame, test.ShouldNotBeNil)
}
