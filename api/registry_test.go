package api

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/sensor"

	"github.com/stretchr/testify/assert"
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

	assert.NotNil(t, ProviderLookup("x"))
	assert.NotNil(t, CameraLookup("x"))
	assert.NotNil(t, ArmLookup("x"))
	assert.NotNil(t, GripperLookup("x"))
	assert.NotNil(t, LidarDeviceLookup("x"))
	assert.NotNil(t, SensorLookup(sensor.DeviceType("x"), "y"))
}
