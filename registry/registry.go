// Package registry operates the global registry of robotic parts.
package registry

import (
	"context"
	"fmt"
	"runtime"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	"github.com/mitchellh/copystructure"
	"go.viam.com/utils/rpc/dialer"
	rpcserver "go.viam.com/utils/rpc/server"

	"go.viam.com/core/base"
	"go.viam.com/core/board"
	"go.viam.com/core/component/motor"
	"go.viam.com/core/config"
	"go.viam.com/core/input"
	"go.viam.com/core/resource"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor"
	"go.viam.com/core/subtype"
)

type (
	// A CreateBase creates a base from a given config.
	CreateBase func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (base.Base, error)

	// A CreateSensor creates a sensor from a given config.
	CreateSensor func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (sensor.Sensor, error)

	// A CreateBoard creates a board from a given config.
	CreateBoard func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (board.Board, error)

	// A CreateMotor creates a motor from a given config.
	CreateMotor func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (motor.Motor, error)

	// A CreateInputController creates an input.Controller from a given config.
	CreateInputController func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (input.Controller, error)

	// A CreateService creates a service from a given config.
	CreateService func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error)
)

// RegDebugInfo represents some runtime information about the registration used
// for debugging purposes.
type RegDebugInfo struct {
	RegistrarLoc string
}

// Base stores a Base constructor function (mandatory)
type Base struct {
	RegDebugInfo
	Constructor CreateBase
}

// Sensor stores a Sensor constructor function (mandatory)
type Sensor struct {
	RegDebugInfo
	Constructor CreateSensor
}

// Board stores a Board constructor function (mandatory)
type Board struct {
	RegDebugInfo
	Constructor CreateBoard
}

// Motor stores a Motor constructor function (mandatory)
type Motor struct {
	RegDebugInfo
	Constructor CreateMotor
}

// InputController stores an input.Controller constructor (mandatory)
type InputController struct {
	RegDebugInfo
	Constructor CreateInputController
}

// Service stores a Service constructor (mandatory) and an attribute converter
type Service struct {
	RegDebugInfo
	Constructor           CreateService
	AttributeMapConverter config.AttributeMapConverter
}

// all registries
var (
	baseRegistry            = map[string]Base{}
	sensorRegistry          = map[sensor.Type]map[string]Sensor{}
	boardRegistry           = map[string]Board{}
	inputControllerRegistry = map[string]InputController{}
	serviceRegistry         = map[config.ServiceType]Service{}
)

func getCallerName() string {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		return details.Name()
	}
	return "unknown"
}

// RegisterBase registers a base model to a creator.
func RegisterBase(model string, creator Base) {
	creator.RegistrarLoc = getCallerName()
	_, old := baseRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two bases with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	baseRegistry[model] = creator
}

// RegisterSensor registers a sensor type and model to a creator.
func RegisterSensor(sensorType sensor.Type, model string, creator Sensor) {
	creator.RegistrarLoc = getCallerName()
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
	creator.RegistrarLoc = getCallerName()
	_, old := boardRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two boards with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	boardRegistry[model] = creator
}

// RegisterInputController registers an input controller model to a creator.
func RegisterInputController(model string, creator InputController) {
	creator.RegistrarLoc = getCallerName()
	_, old := inputControllerRegistry[model]
	if old {
		panic(errors.Errorf("trying to register two input controllers with same model %s", model))
	}
	if creator.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for model %s", model))
	}
	inputControllerRegistry[model] = creator
}

// RegisterService registers a service type to a registration.
func RegisterService(typeName config.ServiceType, registration Service) {
	registration.RegistrarLoc = getCallerName()
	_, old := serviceRegistry[typeName]
	if old {
		panic(errors.Errorf("trying to register two sevices with same type %s", typeName))
	}
	if registration.Constructor == nil {
		panic(errors.Errorf("cannot register a nil constructor for service %s", typeName))
	}
	serviceRegistry[typeName] = registration
}

// BaseLookup looks up a base creator by the given model. nil is returned if
// there is no creator registered.
func BaseLookup(model string) *Base {
	if registration, ok := baseRegistry[model]; ok {
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

// BoardLookup looks up a board creator by the given model. nil is returned if
// there is no creator registered.
func BoardLookup(model string) *Board {
	if registration, ok := boardRegistry[model]; ok {
		return &registration
	}
	return nil
}

// InputControllerLookup looks up an input.Controller creator by the given model. nil is returned if
// there is no creator registered.
func InputControllerLookup(model string) *InputController {
	if registration, ok := inputControllerRegistry[model]; ok {
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

	// A RegisterSubtypeRPCService will register the subtype service to the grpc server
	RegisterSubtypeRPCService func(ctx context.Context, rpcServer rpcserver.Server, subtypeSvc subtype.Service) error

	// A CreateRPCClient will create the client for the resource.
	// TODO: Remove as part of #227
	CreateRPCClient func(conn dialer.ClientConn, name string, logger golog.Logger) interface{}
)

// Component stores a resource constructor (mandatory) and a Frame building function (optional)
type Component struct {
	RegDebugInfo
	Constructor CreateComponent
}

// ResourceSubtype stores subtype-specific functions and clients
type ResourceSubtype struct {
	Reconfigurable     CreateReconfigurable
	RegisterRPCService RegisterSubtypeRPCService
	RPCClient          CreateRPCClient
}

// SubtypeGrpc stores functions necessary for a resource subtype to be accessible through grpc
type SubtypeGrpc struct {
}

// all registries
var (
	componentRegistry = map[string]Component{}
	subtypeRegistry   = map[resource.Subtype]ResourceSubtype{}
)

// RegisterComponent register a creator to its corresponding component and model.
func RegisterComponent(subtype resource.Subtype, model string, creator Component) {
	creator.RegistrarLoc = getCallerName()
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

// RegisterResourceSubtype register a ResourceSubtype to its corresponding component subtype.
func RegisterResourceSubtype(subtype resource.Subtype, creator ResourceSubtype) {
	_, old := subtypeRegistry[subtype]
	if old {
		panic(errors.Errorf("trying to register two of the same component subtype:%s", subtype))
	}
	if creator.Reconfigurable == nil && creator.RegisterRPCService == nil && creator.RPCClient == nil {
		panic(errors.Errorf("cannot register a nil constructor for subtype:%s", subtype))
	}
	subtypeRegistry[subtype] = creator
}

// ResourceSubtypeLookup looks up a ResourceSubtype by the given subtype. nil is returned if
// there is None.
func ResourceSubtypeLookup(subtype resource.Subtype) *ResourceSubtype {
	if registration, ok := subtypeRegistry[subtype]; ok {
		return &registration
	}
	return nil
}

// RegisteredBases returns a copy of the registered bases.
func RegisteredBases() map[string]Base {
	copied, err := copystructure.Copy(baseRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Base)
}

// RegisteredSensors returns a copy of the registered sensors.
func RegisteredSensors() map[sensor.Type]map[string]Sensor {
	copied, err := copystructure.Copy(sensorRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[sensor.Type]map[string]Sensor)
}

// RegisteredBoards returns a copy of the registered boards.
func RegisteredBoards() map[string]Board {
	copied, err := copystructure.Copy(boardRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Board)
}

// RegisteredInputControllers returns a copy of the registered input controllers.
func RegisteredInputControllers() map[string]InputController {
	copied, err := copystructure.Copy(inputControllerRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]InputController)
}

// RegisteredServices returns a copy of the registered services.
func RegisteredServices() map[config.ServiceType]Service {
	copied, err := copystructure.Copy(serviceRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[config.ServiceType]Service)
}

// RegisteredComponents returns a copy of the registered components.
func RegisteredComponents() map[string]Component {
	copied, err := copystructure.Copy(componentRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[string]Component)
}

// RegisteredResourceSubtypes returns a copy of the registered resource subtypes.
func RegisteredResourceSubtypes() map[resource.Subtype]ResourceSubtype {
	copied, err := copystructure.Copy(subtypeRegistry)
	if err != nil {
		panic(err)
	}
	return copied.(map[resource.Subtype]ResourceSubtype)
}
