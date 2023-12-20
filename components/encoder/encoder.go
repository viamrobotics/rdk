// Package encoder implements the encoder component
package encoder

import (
	"context"

	pb "go.viam.com/api/component/encoder/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Encoder]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterEncoderServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.EncoderService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: ticksCount.String(),
	}, newTicksCountCollector)
}

// SubtypeName is a constant that identifies the component resource API string "encoder".
const SubtypeName = "encoder"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// PositionType is an enum representing the encoder's position.
type PositionType byte

// Known encoder position types.
const (
	PositionTypeUnspecified PositionType = iota
	// PositionTypeTicks is for relative encoders
	// that report how far they've gone from a start position.
	PositionTypeTicks
	// PositionTypeDegrees is for absolute encoders
	// that report their position in degrees along the radial axis.
	PositionTypeDegrees
)

func (t PositionType) String() string {
	switch t {
	case PositionTypeTicks:
		return "ticks"
	case PositionTypeDegrees:
		return "degrees"
	case PositionTypeUnspecified:
		fallthrough
	default:
		return "unspecified"
	}
}

// A Encoder turns a position into a signal.
type Encoder interface {
	resource.Resource

	// Position returns the current position in terms of ticks or degrees, and whether it is a relative or absolute position.
	Position(ctx context.Context, positionType PositionType, extra map[string]interface{}) (float64, PositionType, error)

	// ResetPosition sets the current position of the motor to be its new zero position.
	ResetPosition(ctx context.Context, extra map[string]interface{}) error

	// Properties returns a list of all the position types that are supported by a given encoder
	Properties(ctx context.Context, extra map[string]interface{}) (Properties, error)
}

// Named is a helper for getting the named Encoder's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromDependencies is a helper for getting the named encoder from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Encoder, error) {
	return resource.FromDependencies[Encoder](deps, Named(name))
}

// FromRobot is a helper for getting the named encoder from the given Robot.
func FromRobot(r robot.Robot, name string) (Encoder, error) {
	return robot.ResourceFromRobot[Encoder](r, Named(name))
}

// NamesFromRobot is a helper for getting all encoder names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// ToEncoderPositionType takes a GetPositionResponse and returns
// an equivalent PositionType-to-int map.
func ToEncoderPositionType(positionType *pb.PositionType) PositionType {
	if positionType == nil {
		return PositionTypeUnspecified
	}
	if *positionType == pb.PositionType_POSITION_TYPE_ANGLE_DEGREES {
		return PositionTypeDegrees
	}
	if *positionType == pb.PositionType_POSITION_TYPE_TICKS_COUNT {
		return PositionTypeTicks
	}
	return PositionTypeUnspecified
}

// ToProtoPositionType takes a map of PositionType-to-int (indicating
// the PositionType) and converts it to a GetPositionResponse.
func ToProtoPositionType(positionType PositionType) pb.PositionType {
	if positionType == PositionTypeDegrees {
		return pb.PositionType_POSITION_TYPE_ANGLE_DEGREES
	}
	if positionType == PositionTypeTicks {
		return pb.PositionType_POSITION_TYPE_TICKS_COUNT
	}
	return pb.PositionType_POSITION_TYPE_UNSPECIFIED
}
