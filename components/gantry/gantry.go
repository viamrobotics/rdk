// Package gantry defines a robotic gantry with one or multiple axes.
// For more information, see the [gantry component docs].
//
// [gantry component docs]: https://docs.viam.com/components/gantry/
package gantry

import (
	"context"
	"fmt"

	pb "go.viam.com/api/component/gantry/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Gantry]{
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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
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
// For more information, see the [gantry component docs].
//
// Position example:
//
//	myGantry, err := gantry.GetResource(machine, "my_gantry")
//
//	// Get the current positions of the axes of the gantry in millimeters.
//	position, err := myGantry.Position(context.Background(), nil)
//
// For more information, see the [Position method docs].
//
// MoveToPosition example:
//
//	myGantry, err := gantry.GetResource(machine, "my_gantry")
//
//	// Create a list of positions for the axes of the gantry to move to.
//	// Assume in this example that the gantry is multi-axis, with 3 axes.
//	examplePositions := []float64{1, 2, 3}
//
//	exampleSpeeds := []float64{3, 9, 12}
//
//	// Move the axes of the gantry to the positions specified.
//	myGantry.MoveToPosition(context.Background(), examplePositions, exampleSpeeds, nil)
//
// For more information, see the [MoveToPosition method docs].
//
// Lengths example:
//
//	myGantry, err := gantry.GetResource(machine, "my_gantry")
//
//	// Get the lengths of the axes of the gantry in millimeters.
//	lengths_mm, err := myGantry.Lengths(context.Background(), nil)
//
// For more information, see the [Lengths method docs].
//
// Home example:
//
//	myGantry, err := gantry.GetResource(machine, "my_gantry")
//
//	myGantry.Home(context.Background(), nil)
//
// For more information, see the [Home method docs].
//
// [gantry component docs]: https://docs.viam.com/dev/reference/apis/components/gantry/
// [Position method docs]: https://docs.viam.com/dev/reference/apis/components/gantry/#getposition
// [MoveToPosition method docs]: https://docs.viam.com/dev/reference/apis/components/gantry/#movetoposition
// [Lengths method docs]: https://docs.viam.com/dev/reference/apis/components/gantry/#getlengths
// [Home method docs]: https://docs.viam.com/dev/reference/apis/components/gantry/#home
type Gantry interface {
	resource.Resource
	resource.Actuator
	framesystem.InputEnabled

	// Position returns the position in meters.
	Position(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// MoveToPosition is in meters.
	// This will block until done or a new operation cancels this one.
	MoveToPosition(ctx context.Context, positionsMm, speedsMmPerSec []float64, extra map[string]interface{}) error

	// Lengths is the length of gantries in meters.
	Lengths(ctx context.Context, extra map[string]interface{}) ([]float64, error)

	// Home runs the homing sequence of the gantry and returns true once completed.
	Home(ctx context.Context, extra map[string]interface{}) (bool, error)
}

// GetResource is a helper for getting the named Gantry from either a collection of dependencies
// or the given robot.
func GetResource(src any, name string) (Gantry, error) {
	switch v := src.(type) {
	case resource.Dependencies:
		return resource.FromDependencies[Gantry](v, Named(name))
	case robot.Robot:
		return robot.ResourceFromRobot[Gantry](v, Named(name))
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}

// NamesFromRobot is a helper for getting all gantry names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}
