package registry

import (
	"context"
	"testing"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	pf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (robot.Provider, error) {
		return nil, nil
	}

	af := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error) {
		return nil, nil
	}

	cf := func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gostream.ImageSource, error) {
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

	RegisterProvider("x", pf)
	RegisterCamera("x", cf)
	RegisterArm("x", af)
	RegisterGripper("x", gf)
	RegisterLidar("x", lf)
	RegisterSensor(sensor.Type("x"), "y", sf)

	test.That(t, ProviderLookup("x"), test.ShouldNotBeNil)
	test.That(t, CameraLookup("x"), test.ShouldNotBeNil)
	test.That(t, ArmLookup("x"), test.ShouldNotBeNil)
	test.That(t, GripperLookup("x"), test.ShouldNotBeNil)
	test.That(t, LidarLookup("x"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.Type("x"), "y"), test.ShouldNotBeNil)
}
