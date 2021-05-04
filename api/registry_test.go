package api

import (
	"context"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/sensor"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/test"
)

func TestRegistry(t *testing.T) {
	pf := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Provider, error) {
		return nil, nil
	}

	af := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Arm, error) {
		return nil, nil
	}

	cf := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (gostream.ImageSource, error) {
		return nil, nil
	}

	gf := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Gripper, error) {
		return nil, nil
	}

	lf := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (lidar.Device, error) {
		return nil, nil
	}

	sf := func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (sensor.Device, error) {
		return nil, nil
	}

	RegisterProvider("x", pf)
	RegisterCamera("x", cf)
	RegisterArm("x", af)
	RegisterGripper("x", gf)
	RegisterLidarDevice("x", lf)
	RegisterSensor(sensor.DeviceType("x"), "y", sf)

	test.That(t, ProviderLookup("x"), test.ShouldNotBeNil)
	test.That(t, CameraLookup("x"), test.ShouldNotBeNil)
	test.That(t, ArmLookup("x"), test.ShouldNotBeNil)
	test.That(t, GripperLookup("x"), test.ShouldNotBeNil)
	test.That(t, LidarDeviceLookup("x"), test.ShouldNotBeNil)
	test.That(t, SensorLookup(sensor.DeviceType("x"), "y"), test.ShouldNotBeNil)
}
