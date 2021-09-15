// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/arm"
	"go.viam.com/core/base"
	"go.viam.com/core/camera"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
)

type (
	// A CreateCamera creates a camera from a given config.
	CreateCamera func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error)

	// A CreateArm creates an arm from a given config.
	CreateArm func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (arm.Arm, error)

	// A CreateGripper creates a gripper from a given config.
	CreateGripper func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (gripper.Gripper, error)

	// A CreateBase creates a base from a given config.
	CreateBase func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error)

	// A CreateLidar creates a lidar from a given config.
	CreateLidar func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (lidar.Lidar, error)

	// A CreateSensor creates a sensor from a given config.
	CreateSensor func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error)
)

// all registries
var (
	cameraRegistry  = map[string]CreateCamera{}
	armRegistry     = map[string]CreateArm{}
	gripperRegistry = map[string]CreateGripper{}
	baseRegistry    = map[string]CreateBase{}
	lidarRegistry   = map[string]CreateLidar{}
	sensorRegistry  = map[sensor.Type]map[string]CreateSensor{}
)

// RegisterCamera register a camera model to a creator.
func RegisterCamera(model string, creator CreateCamera) {
	_, old := cameraRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two cameras with same model %s", model))
	}
	cameraRegistry[model] = creator
}

// RegisterArm register an arm model to a creator.
func RegisterArm(model string, creator CreateArm) {
	_, old := armRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two arms with same model %s", model))
	}
	armRegistry[model] = creator
}

// RegisterGripper register a gripper model to a creator.
func RegisterGripper(model string, creator CreateGripper) {
	_, old := gripperRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two grippers with same model %s", model))
	}
	gripperRegistry[model] = creator
}

// RegisterBase register a base model to a creator.
func RegisterBase(model string, creator CreateBase) {
	_, old := baseRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two bases with same model %s", model))
	}
	baseRegistry[model] = creator
}

// RegisterLidar register a lidar model to a creator.
func RegisterLidar(model string, creator CreateLidar) {
	_, old := lidarRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two lidars with same model %s", model))
	}
	lidarRegistry[model] = creator
}

// RegisterSensor register a sensor type and model to a creator.
func RegisterSensor(sensorType sensor.Type, model string, creator CreateSensor) {
	if _, ok := sensorRegistry[sensorType]; !ok {
		sensorRegistry[sensorType] = map[string]CreateSensor{}
	}
	_, old := sensorRegistry[sensorType][model]
	if old {
		panic(errors.Errorf("trying to register two sensors with same model %s", model))
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

// BaseLookup looks up a base creator by the given model. nil is returned if
// there is no creator registered.
func BaseLookup(model string) CreateBase {
	return baseRegistry[model]
}

// LidarLookup looks up a lidar creator by the given model. nil is returned if
// there is no creator registered.
func LidarLookup(model string) CreateLidar {
	return lidarRegistry[model]
}

// SensorLookup looks up a sensor creator by the given model. nil is returned if
// there is no creator registered.
func SensorLookup(sensorType sensor.Type, model string) CreateSensor {
	subTyped, ok := sensorRegistry[sensorType]
	if !ok {
		return nil
	}
	return subTyped[model]
}
