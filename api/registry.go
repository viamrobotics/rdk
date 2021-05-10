package api

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/sensor"
)

type (
	// A CreateProvider creates a provider from a given config.
	CreateProvider func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Provider, error)

	// A CreateCamera creates a camera from a given config.
	CreateCamera func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (gostream.ImageSource, error)

	// A CreateArm creates an arm from a given config.
	CreateArm func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Arm, error)

	// A CreateGripper creates a gripper from a given config.
	CreateGripper func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Gripper, error)

	// A CreateBase creates a base from a given config.
	CreateBase func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Base, error)

	// A CreateLidarDevice creates a lidar device from a given config.
	CreateLidarDevice func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (lidar.Device, error)

	// A CreateSensor creates a sensor from a given config.
	CreateSensor func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (sensor.Device, error)
)

// all registries
var (
	cameraRegistry      = map[string]CreateCamera{}
	armRegistry         = map[string]CreateArm{}
	gripperRegistry     = map[string]CreateGripper{}
	providerRegistry    = map[string]CreateProvider{}
	baseRegistry        = map[string]CreateBase{}
	lidarDeviceRegistry = map[string]CreateLidarDevice{}
	sensorRegistry      = map[sensor.DeviceType]map[string]CreateSensor{}
)

// RegisterCamera register a camera model to a creator.
func RegisterCamera(model string, creator CreateCamera) {
	_, old := cameraRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two cameras with same model %s", model))
	}
	cameraRegistry[model] = creator
}

// RegisterArm register an arm model to a creator.
func RegisterArm(model string, creator CreateArm) {
	_, old := armRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two arms with same model %s", model))
	}
	armRegistry[model] = creator
}

// RegisterGripper register a gripper model to a creator.
func RegisterGripper(model string, creator CreateGripper) {
	_, old := gripperRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two grippers with same model %s", model))
	}
	gripperRegistry[model] = creator
}

// RegisterProvider register a provider model to a creator.
func RegisterProvider(model string, creator CreateProvider) {
	_, old := providerRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two providers with same model %s", model))
	}
	providerRegistry[model] = creator
}

// RegisterBase register a base model to a creator.
func RegisterBase(model string, creator CreateBase) {
	_, old := baseRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two bases with same model %s", model))
	}
	baseRegistry[model] = creator
}

// RegisterLidarDevice register a lidar device model to a creator.
func RegisterLidarDevice(model string, creator CreateLidarDevice) {
	_, old := lidarDeviceRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two lidar devices with same model %s", model))
	}
	lidarDeviceRegistry[model] = creator
}

// RegisterSensor register a sensor type and model to a creator.
func RegisterSensor(sensorType sensor.DeviceType, model string, creator CreateSensor) {
	if _, ok := sensorRegistry[sensorType]; !ok {
		sensorRegistry[sensorType] = map[string]CreateSensor{}
	}
	_, old := sensorRegistry[sensorType][model]
	if old {
		panic(fmt.Errorf("trying to register two sensors with same model %s", model))
	}
	sensorRegistry[sensorType][model] = creator
}

// CameraLookup looks up a camera creator by the given model. nil is returned if
// there is no creator registered.
func CameraLookup(model string) CreateCamera {
	return cameraRegistry[model]
}

// ArmLookup looks up an arm creator by the given model. nil is returned if
// there is no creator registered.
func ArmLookup(model string) CreateArm {
	return armRegistry[model]
}

// GripperLookup looks up a gripper creator by the given model. nil is returned if
// there is no creator registered.
func GripperLookup(model string) CreateGripper {
	return gripperRegistry[model]
}

// ProviderLookup looks up a provider creator by the given model. nil is returned if
// there is no creator registered.
func ProviderLookup(model string) CreateProvider {
	return providerRegistry[model]
}

// BaseLookup looks up a base creator by the given model. nil is returned if
// there is no creator registered.
func BaseLookup(model string) CreateBase {
	return baseRegistry[model]
}

// LidarDeviceLookup looks up a lidar device creator by the given model. nil is returned if
// there is no creator registered.
func LidarDeviceLookup(model string) CreateLidarDevice {
	return lidarDeviceRegistry[model]
}

// SensorLookup looks up a sensor creator by the given model. nil is returned if
// there is no creator registered.
func SensorLookup(sensorType sensor.DeviceType, model string) CreateSensor {
	subTyped, ok := sensorRegistry[sensorType]
	if !ok {
		return nil
	}
	return subTyped[model]
}
