package gantry

import (
	"context"

	pb "go.viam.com/api/component/gantry/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Gantry]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterGantryServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.GantryService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: position.String(),
	}, newPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: lengths.String(),
	}, newLengthsCollector)
}

// SubtypeName is a constant that identifies the component resource API string "gantry".
const SubtypeName = "gantry"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Gantry's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// Gantry is used for controlling gantries of N axis.
type Gantry interface {
	resource.Resource
	resource.Actuator
	referenceframe.ModelFramer
	referenceframe.InputEnabled

	// Position returns the position in meters
	Position(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// MoveToPosition is in meters
	// This will block until done or a new operation cancels this one
	MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error

	// Lengths is the length of gantries in meters
	Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// Home runs the homing sequence of the gantry and returns true once completed
	Home(ctx context.Context, extra map[string]interface{}) (bool, error)
}

// FromDependencies is a helper for getting the named gantry from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Gantry, error) {
	return resource.FromDependencies[Gantry](deps, Named(name))
}

// FromRobot is a helper for getting the named gantry from the given Robot.
func FromRobot(r robot.Robot, name string) (Gantry, error) {
	return robot.ResourceFromRobot[Gantry](r, Named(name))
}

// NamesFromRobot is a helper for getting all gantry names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// CreateStatus creates a status from the gantry.
func CreateStatus(ctx context.Context, g Gantry) (*pb.Status, error) {
	positions, err := g.Position(ctx, nil)
	if err != nil {
		return nil, err
	}

	lengths, err := g.Lengths(ctx, nil)
	if err != nil {
		return nil, err
	}
	isMoving, err := g.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{PositionsMm: positions, LengthsMm: lengths, IsMoving: isMoving}, nil
}
