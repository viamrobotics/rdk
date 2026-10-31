// Package ptz defines a pan-tilt-zoom camera interface.
package ptz

import (
	"context"

	pb "go.viam.com/api/component/ptz/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[PTZ]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterPTZServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.PTZService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string.
const SubtypeName = "ptz"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named PTZ's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Status represents the current PTZ position and movement state.
type Status struct {
	Position       *pb.Pose
	PanTiltStatus  pb.PTZMoveStatus
	ZoomStatus     pb.PTZMoveStatus
	UtcTime        *timestamppb.Timestamp
}

// Capabilities represents standardized PTZ capabilities for a device.
type Capabilities struct {
	MoveCapabilities []*pb.PTZMoveCapability
	SupportsStatus   *bool
	SupportsStop     *bool
}

// MoveCommand represents a PTZ move request with a single command populated.
type MoveCommand struct {
	Continuous *pb.ContinuousMove
	Relative   *pb.RelativeMove
	Absolute   *pb.AbsoluteMove
}

// A PTZ represents a pan-tilt-zoom capable camera.
type PTZ interface {
	resource.Resource

	// GetStatus returns the current PTZ position and movement status.
	GetStatus(ctx context.Context, extra map[string]interface{}) (*Status, error)

	// GetCapabilities returns standardized PTZ capabilities for this device.
	GetCapabilities(ctx context.Context, extra map[string]interface{}) (*Capabilities, error)

	// Stop halts any ongoing PTZ movement. If panTilt or zoom are nil, the
	// device default is used.
	Stop(ctx context.Context, panTilt, zoom *bool, extra map[string]interface{}) error

	// Move executes a PTZ movement command (continuous, relative, or absolute).
	Move(ctx context.Context, cmd *MoveCommand, extra map[string]interface{}) error
}

// Deprecated: FromRobot is a helper for getting the named PTZ from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (PTZ, error) {
	return robot.ResourceFromRobot[PTZ](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named PTZ from a collection of dependencies.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (PTZ, error) {
	return resource.FromDependencies[PTZ](deps, Named(name))
}

// FromProvider is a helper for getting the named PTZ from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (PTZ, error) {
	return resource.FromProvider[PTZ](provider, Named(name))
}

// NamesFromRobot is a helper for getting all PTZ names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}
