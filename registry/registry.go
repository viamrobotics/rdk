// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"
	"fmt"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/camera"
	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	"go.viam.com/core/lidar"
	"go.viam.com/core/motor"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/servo"
)

type (
	// A CreateCamera creates a camera from a given config.
	CreateCamera func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (camera.Camera, error)

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

	// A CreateBoard creates a board from a given config.
	CreateBoard func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (board.Board, error)

	// A CreateServo creates a servo from a given config.
	CreateServo func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (servo.Servo, error)

	// A CreateMotor creates a motor from a given config.
	CreateMotor func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error)

	// A CreateService creates a servoce from a given config.
	CreateService func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error)
)

// Camera stores a Camera constructor (mandatory) and a Frame building function (optional)
type Camera struct {
	Constructor CreateCamera
	Frame       CreateFrame
}

// Gripper stores a Gripper constructor (mandatory) and a Frame building function (optional)
type Gripper struct {
	Constructor CreateGripper
	Frame       CreateFrame
}

// Base stores a Base constructor (mandatory) and a Frame building function (optional)
type Base struct {
	Constructor CreateBase
	Frame       CreateFrame
}

// Lidar stores a Lidar constructor (mandatory) and a Frame building function (optional)
type Lidar struct {
	Constructor CreateLidar
	Frame       CreateFrame
}

// Sensor stores a Sensor constructor (mandatory) and a Frame building function (optional)
type Sensor struct {
	Constructor CreateSensor
	Frame       CreateFrame
}

// Board stores a Board constructor (mandatory) and a Frame building function (optional)
type Board struct {
	Constructor CreateBoard
	Frame       CreateFrame
}

// Servo stores a Servo constructor (mandatory) and a Frame building function (optional)
type Servo struct {
	Constructor CreateServo
	Frame       CreateFrame
}

// Motor stores a Motor constructor (mandatory) and a Frame building function (optional)
type Motor struct {
	Constructor CreateMotor
	Frame       CreateFrame
}

// Service stores a Service constructor (mandatory) and an attribute converter
type Service struct {
	Constructor           CreateService
	AttributeMapConverter config.AttributeMapConverter
}

// all registries
var (
	cameraRegistry  = map[string]Camera{}
	gripperRegistry = map[string]Gripper{}
	baseRegistry    = map[string]Base{}
	lidarRegistry   = map[string]Lidar{}
	sensorRegistry  = map[sensor.Type]map[string]Sensor{}
	boardRegistry   = map[string]Board{}
	servoRegistry   = map[string]Servo{}
	motorRegistry   = map[string]Motor{}
	serviceRegistry = map[config.ServiceType]Service{}
)

// RegisterCamera registers a camera model to a creator.
func RegisterCamera(model string, creator Camera) {
	_, old := cameraRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two cameras with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	cameraRegistry[model] = creator
}

// RegisterGripper registers a gripper model to a creator.
func RegisterGripper(model string, creator Gripper) {
	_, old := gripperRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two grippers with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	gripperRegistry[model] = creator
}

// RegisterBase registers a base model to a creator.
func RegisterBase(model string, creator Base) {
	_, old := baseRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two bases with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	baseRegistry[model] = creator
}

// RegisterLidar registers a lidar model to a creator.
func RegisterLidar(model string, creator Lidar) {
	_, old := lidarRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two lidars with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	lidarRegistry[model] = creator
}

// RegisterSensor registers a sensor type and model to a creator.
func RegisterSensor(sensorType sensor.Type, model string, creator Sensor) {
	if _, ok := sensorRegistry[sensorType]; !ok {
		sensorRegistry[sensorType] = make(map[string]Sensor)
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

// RegisterBoard registers a board model to a creator.
func RegisterBoard(model string, creator Board) {
	_, old := boardRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two boards with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	boardRegistry[model] = creator
}

// RegisterServo registers a servo model to a creator.
func RegisterServo(model string, creator Servo) {
	_, old := servoRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two servos with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	servoRegistry[model] = creator
}

// RegisterMotor registers a motor model to a creator.
func RegisterMotor(model string, creator Motor) {
	_, old := motorRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two motors with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	motorRegistry[model] = creator
}

// RegisterService registers a service type to a registration.
func RegisterService(typeName config.ServiceType, registration Service) {
	_, old := serviceRegistry[typeName]
	if old {
		panic(errors.Errorf("trying to register two sevices with same type %s", typeName))
	}
	if registration.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for service %s", typeName))
	}
	if registration.AttributeMapConverter != nil {
		config.RegisterServiceAttributeMapConverter(typeName, registration.AttributeMapConverter)
	}
	serviceRegistry[typeName] = registration
}

// CameraLookup looks up a camera creator by the given model. nil is returned if
// there is no creator registered.
func CameraLookup(model string) *Camera {
	if registration, ok := cameraRegistry[model]; ok {
		return &registration
	}
	return nil
}

// GripperLookup looks up a gripper creator by the given model. nil is returned if
// there is no creator registered.
func GripperLookup(model string) *Gripper {
	if registration, ok := gripperRegistry[model]; ok {
		return &registration
	}
	return nil
}

// BaseLookup looks up a base creator by the given model. nil is returned if
// there is no creator registered.
func BaseLookup(model string) *Base {
	if registration, ok := baseRegistry[model]; ok {
		return &registration
	}
	return nil
}

// LidarLookup looks up a lidar creator by the given model. nil is returned if
// there is no creator registered.
func LidarLookup(model string) *Lidar {
	if registration, ok := lidarRegistry[model]; ok {
		return &registration
	}
	return nil
}

// SensorLookup looks up a sensor creator by the given model. nil is returned if
// there is no creator registered.
func SensorLookup(sensorType sensor.Type, model string) *Sensor {
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
	case config.ComponentTypeBase:
		registration := BaseLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeArm:
		rName := comp.ResourceName()
		registration := ComponentLookup(rName.Subtype, comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeGripper:
		registration := GripperLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeCamera:
		registration := CameraLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeLidar:
		registration := LidarLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeSensor:
		if comp.SubType == "" {
			return nil, false
		}
		registration := SensorLookup(sensor.Type(comp.SubType), comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeBoard:
		registration := BoardLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeServo:
		registration := ServoLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	case config.ComponentTypeMotor:
		registration := MotorLookup(comp.Model)
		if registration == nil || registration.Frame == nil {
			return nil, false
		}
		return registration.Frame, true
	default:
		return nil, false
	}
}

// BoardLookup looks up a board creator by the given model. nil is returned if
// there is no creator registered.
func BoardLookup(model string) *Board {
	if registration, ok := boardRegistry[model]; ok {
		return &registration
	}
	return nil
}

// ServoLookup looks up a servo creator by the given model. nil is returned if
// there is no creator registered.
func ServoLookup(model string) *Servo {
	if registration, ok := servoRegistry[model]; ok {
		return &registration
	}
	return nil
}

// MotorLookup looks up a motor creator by the given model. nil is returned if
// there is no creator registered.
func MotorLookup(model string) *Motor {
	if registration, ok := motorRegistry[model]; ok {
		return &registration
	}
	return nil
}

// ServiceLookup looks up a service registration by the given type. nil is returned if
// there is no registration.
func ServiceLookup(typeName config.ServiceType) *Service {
	if registration, ok := serviceRegistry[typeName]; ok {
		return &registration
	}
	return nil
}

// core api v2 implementation starts here

type (
	// A CreateComponent creates a resource from a given config.
	CreateComponent func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error)

	// A CreateReconfigurable makes a reconfigurable resource from a given resource.
	CreateReconfigurable func(resource interface{}) (resource.Reconfigurable, error)
)

// Component stores a resource constructor (mandatory) and a Frame building function (optional)
type Component struct {
	Constructor CreateComponent
	Frame       CreateFrame
}

// ComponentSubtype stores a reconfigurable resource creator
type ComponentSubtype struct {
	Reconfigurable CreateReconfigurable
}

// all registries
var (
	componentRegistry        = map[string]Component{}
	componentSubtypeRegistry = map[resource.Subtype]ComponentSubtype{}
)

// RegisterComponent register a creator to its corresponding component and model.
func RegisterComponent(subtype resource.Subtype, model string, creator Component) {
	qName := fmt.Sprintf("%s/%s", subtype, model)
	_, old := componentRegistry[qName]
	if old {
		panic(errors.Errorf("trying to register two resources with same subtype:%s, model:%s", subtype, model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype:%s, model:%s", subtype, model))
	}
	componentRegistry[qName] = creator
}

// ComponentLookup looks up a creator by the given subtype and model. nil is returned if
// there is no creator registered.
func ComponentLookup(subtype resource.Subtype, model string) *Component {
	qName := fmt.Sprintf("%s/%s", subtype, model)
	if registration, ok := componentRegistry[qName]; ok {
		return &registration
	}
	return nil
}

// RegisterComponentSubtype register a ComponentSubtype to its corresponding component subtype.
func RegisterComponentSubtype(subtype resource.Subtype, creator ComponentSubtype) {
	_, old := componentSubtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same component subtype:%s", subtype))
	}
	if creator.Reconfigurable == nil {
		panic(errors.Errorf("cannot register a nil Reconfigurable constructor for subtype:%s", subtype))
	}
	componentSubtypeRegistry[subtype] = creator
}

// ComponentSubtypeLookup looks up a ComponentSubtype by the given subtype. nil is returned if
// there is None.
func ComponentSubtypeLookup(subtype resource.Subtype) *ComponentSubtype {
	if registration, ok := componentSubtypeRegistry[subtype]; ok {
		return &registration
	}
	return nil
}

// TODO: currently here because of import cycles. get rid of this block at conclusion of Core v2 migration.
//these registrations should happen in the subtype's go package instead.
func init() {
	RegisterComponentSubtype(arm.Subtype, ComponentSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return arm.WrapWithReconfigurable(r)
		},
	})
}
