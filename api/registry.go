package api

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/edaniels/gostream"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/sensor"
)

type CreateProvider func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Provider, error)
type CreateCamera func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (gostream.ImageSource, error)
type CreateArm func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Arm, error)
type CreateGripper func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Gripper, error)
type CreateBase func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (Base, error)
type CreateLidarDevice func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (lidar.Device, error)
type CreateSensor func(ctx context.Context, r Robot, config ComponentConfig, logger golog.Logger) (sensor.Device, error)

var (
	cameraRegistry      = map[string]CreateCamera{}
	armRegistry         = map[string]CreateArm{}
	gripperRegistry     = map[string]CreateGripper{}
	providerRegistry    = map[string]CreateProvider{}
	baseRegistry        = map[string]CreateBase{}
	lidarDeviceRegistry = map[string]CreateLidarDevice{}
	sensorRegistry      = map[sensor.DeviceType]map[string]CreateSensor{}
)

func RegisterCamera(model string, f CreateCamera) {
	_, old := cameraRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two cameras with same model %s", model))
	}
	cameraRegistry[model] = f
}

func RegisterArm(model string, f CreateArm) {
	_, old := armRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two arms with same model %s", model))
	}
	armRegistry[model] = f
}

func RegisterGripper(model string, f CreateGripper) {
	_, old := gripperRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two grippers with same model %s", model))
	}
	gripperRegistry[model] = f
}

func RegisterProvider(model string, f CreateProvider) {
	_, old := providerRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two providers with same model %s", model))
	}
	providerRegistry[model] = f
}

func RegisterBase(model string, f CreateBase) {
	_, old := baseRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two bases with same model %s", model))
	}
	baseRegistry[model] = f
}

func RegisterLidarDevice(model string, f CreateLidarDevice) {
	_, old := lidarDeviceRegistry[model]
	if old {
		panic(fmt.Errorf("trying to register two lidar devices with same model %s", model))
	}
	lidarDeviceRegistry[model] = f
}

func RegisterSensor(sensorType sensor.DeviceType, model string, f CreateSensor) {
	if _, ok := sensorRegistry[sensorType]; !ok {
		sensorRegistry[sensorType] = map[string]CreateSensor{}
	}
	_, old := sensorRegistry[sensorType][model]
	if old {
		panic(fmt.Errorf("trying to register two sensors with same model %s", model))
	}
	sensorRegistry[sensorType][model] = f
}

func CameraLookup(model string) CreateCamera {
	return cameraRegistry[model]
}

func ArmLookup(model string) CreateArm {
	return armRegistry[model]
}

func GripperLookup(model string) CreateGripper {
	return gripperRegistry[model]
}

func ProviderLookup(model string) CreateProvider {
	return providerRegistry[model]
}

func BaseLookup(model string) CreateBase {
	return baseRegistry[model]
}

func LidarDeviceLookup(model string) CreateLidarDevice {
	return lidarDeviceRegistry[model]
}

func SensorLookup(sensorType sensor.DeviceType, model string) CreateSensor {
	subTyped, ok := sensorRegistry[sensorType]
	if !ok {
		return nil
	}
	return subTyped[model]
}
