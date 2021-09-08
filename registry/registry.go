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
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
)

type (
	// A CreateProvider creates a provider from a given config.
	CreateProvider func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (robot.Provider, error)

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

	// A CreateFrame creates a frame from a given config.
	CreateFrame func(name string) (referenceframe.Frame, error)
)

// ProviderRegistration stores a Provider constructor (mandatory) and a Frame building function (optional)
type ProviderRegistration struct {
	Constructor CreateProvider
	Frame       CreateFrame
}

// CameraRegistration stores a Camera constructor (mandatory) and a Frame building function (optional)
type CameraRegistration struct {
	Constructor CreateCamera
	Frame       CreateFrame
}

// ArmRegistration stores an Arm constructor (mandatory) and a Frame building function (optional)
type ArmRegistration struct {
	Constructor CreateArm
	Frame       CreateFrame
}

// GripperRegistration stores a Gripper constructor (mandatory) and a Frame building function (optional)
type GripperRegistration struct {
	Constructor CreateGripper
	Frame       CreateFrame
}

// BaseRegistration stores a Base constructor (mandatory) and a Frame building function (optional)
type BaseRegistration struct {
	Constructor CreateBase
	Frame       CreateFrame
}

// LidarRegistration stores a Lidar constructor (mandatory) and a Frame building function (optional)
type LidarRegistration struct {
	Constructor CreateLidar
	Frame       CreateFrame
}

// SensorRegistration stores a Sensor constructor (mandatory) and a Frame building function (optional)
type SensorRegistration struct {
	Constructor CreateSensor
	Frame       CreateFrame
}

// all registries
var (
	cameraRegistry   = map[string]CameraRegistration{}
	armRegistry      = map[string]ArmRegistration{}
	gripperRegistry  = map[string]GripperRegistration{}
	providerRegistry = map[string]ProviderRegistration{}
	baseRegistry     = map[string]BaseRegistration{}
	lidarRegistry    = map[string]LidarRegistration{}
	sensorRegistry   = map[sensor.Type]map[string]SensorRegistration{}
)

// RegisterCamera register a camera model to a creator.
func RegisterCamera(model string, creator CameraRegistration) {
	_, old := cameraRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two cameras with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	cameraRegistry[model] = creator
}

// RegisterArm register an arm model to a creator.
func RegisterArm(model string, creator ArmRegistration) {
	_, old := armRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two arms with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	armRegistry[model] = creator
}

// RegisterGripper register a gripper model to a creator.
func RegisterGripper(model string, creator GripperRegistration) {
	_, old := gripperRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two grippers with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	gripperRegistry[model] = creator
}

// RegisterProvider register a provider model to a creator.
func RegisterProvider(model string, creator ProviderRegistration) {
	_, old := providerRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two providers with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	providerRegistry[model] = creator
}

// RegisterBase register a base model to a creator.
func RegisterBase(model string, creator BaseRegistration) {
	_, old := baseRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two bases with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	baseRegistry[model] = creator
}

// RegisterLidar register a lidar model to a creator.
func RegisterLidar(model string, creator LidarRegistration) {
	_, old := lidarRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two lidars with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	lidarRegistry[model] = creator
}

// RegisterSensor register a sensor type and model to a creator.
func RegisterSensor(sensorType sensor.Type, model string, creator SensorRegistration) {
	if _, ok := sensorRegistry[sensorType]; !ok {
		sensorRegistry[sensorType] = make(map[string]SensorRegistration)
	}
	_, old := sensorRegistry[sensorType][model]
	if old {
		panic(errors.Errorf("trying to register two sensors with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	sensorRegistry[sensorType][model] = creator
}

// CameraLookup looks up a camera creator by the given model. nil is returned if
// there is no creator registered.
func CameraLookup(model string) *CameraRegistration {
	if registration, ok := cameraRegistry[model]; ok {
		return &registration
	}
	return nil
}

// ArmLookup looks up an arm creator by the given model. nil is returned if
// there is no creator registered.
func ArmLookup(model string) *ArmRegistration {
	if registration, ok := armRegistry[model]; ok {
		return &registration
	}
	return nil
}

// GripperLookup looks up a gripper creator by the given model. nil is returned if
// there is no creator registered.
func GripperLookup(model string) *GripperRegistration {
	if registration, ok := gripperRegistry[model]; ok {
		return &registration
	}
	return nil
}

// ProviderLookup looks up a provider creator by the given model. nil is returned if
// there is no creator registered.
func ProviderLookup(model string) *ProviderRegistration {
	if registration, ok := providerRegistry[model]; ok {
		return &registration
	}
	return nil
}

// BaseLookup looks up a base creator by the given model. nil is returned if
// there is no creator registered.
func BaseLookup(model string) *BaseRegistration {
	if registration, ok := baseRegistry[model]; ok {
		return &registration
	}
	return nil
}

// LidarLookup looks up a lidar creator by the given model. nil is returned if
// there is no creator registered.
func LidarLookup(model string) *LidarRegistration {
	if registration, ok := lidarRegistry[model]; ok {
		return &registration
	}
	return nil
}

// SensorLookup looks up a sensor creator by the given model. nil is returned if
// there is no creator registered.
func SensorLookup(sensorType sensor.Type, model string) *SensorRegistration {
	subTyped, ok := sensorRegistry[sensorType]
	if !ok {
		return nil
	}
	if registration, ok := subTyped[model]; ok {
		return &registration
	}
	return nil
}

// FrameLookup returns the FrameCreate function and a true bool if a frame is registered for the given component.
// Otherwise it returns nil and false.
func FrameLookup(comp *config.Component) (CreateFrame, bool) {
	switch comp.Type {
	case config.ComponentTypeProvider:
		registration := ProviderLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeBase:
		registration := BaseLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeArm:
		registration := ArmLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeGripper:
		registration := GripperLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeCamera:
		registration := CameraLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeLidar:
		registration := LidarLookup(comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeSensor:
		if comp.SubType == "" {
			return nil, false
		}
		registration := SensorLookup(sensor.Type(comp.SubType), comp.Model)
		if registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	default:
		return nil, false
	}
}
