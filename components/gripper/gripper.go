// Package gripper defines a robotic gripper.
// For more information, see the [gripper component docs].
//
// [gripper component docs]: https://docs.viam.com/components/gripper/
package gripper

import (
	"context"

	pb "go.viam.com/api/component/gripper/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Gripper]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterGripperServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.GripperService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
}

// SubtypeName is a constant that identifies the component resource API string.
const SubtypeName = "gripper"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named grippers's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// HoldingStatus represents whether the gripper is currently holding onto
// an object as well as any additional contextual information (stored in `Meta`).
type HoldingStatus struct {
	IsHoldingSomething bool
	Meta               map[string]interface{}
}

// A Gripper represents a physical robotic gripper.
// For more information, see the [gripper component docs].
//
// Open example:
//
//	myGripper, err := gripper.FromProvider(machine, "my_gripper")
//
//	// Open the gripper.
//	err := myGripper.Open(context.Background(), nil)
//
// For more information, see the [Open method docs].
//
// Grab example:
//
//	myGripper, err := gripper.FromProvider(machine, "my_gripper")
//
//	// Grab with the gripper.
//	grabbed, err := myGripper.Grab(context.Background(), nil)
//
// For more information, see the [Grab method docs].
//
// [gripper component docs]: https://docs.viam.com/dev/reference/apis/components/gripper/
// [Open method docs]: https://docs.viam.com/dev/reference/apis/components/gripper/#open
// [Grab method docs]: https://docs.viam.com/dev/reference/apis/components/gripper/#grab
type Gripper interface {
	resource.Resource
	resource.Shaped
	resource.Actuator
	framesystem.InputEnabled

	// Open opens the gripper.
	// This will block until done or a new operation cancels this one.
	Open(ctx context.Context, extra map[string]interface{}) error

	// Grab makes the gripper grab.
	// returns true if we grabbed something.
	// This will block until done or a new operation cancels this one.
	Grab(ctx context.Context, extra map[string]interface{}) (bool, error)

	// IsHoldingSomething returns whether the gripper is currently holding onto an object.
	IsHoldingSomething(ctx context.Context, extra map[string]interface{}) (HoldingStatus, error)
}

// Deprecated: FromRobot is a helper for getting the named Gripper from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Gripper, error) {
	return robot.ResourceFromRobot[Gripper](r, Named(name))
}

// Deprecated: FromDependencies is a helper for getting the named gripper from a collection of
// dependencies. Use FromProvider instead.
//
//nolint:revive // ignore exported comment check.
func FromDependencies(deps resource.Dependencies, name string) (Gripper, error) {
	return resource.FromDependencies[Gripper](deps, Named(name))
}

// FromProvider is a helper for getting the named Gripper from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Gripper, error) {
	return resource.FromProvider[Gripper](provider, Named(name))
}

// NamesFromRobot is a helper for getting all gripper names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// MakeModel is a helper function that creates a zero DoF Model for a gripper from a list of its geometries.
func MakeModel(name string, geometries []spatialmath.Geometry) (referenceframe.Model, error) {
	if len(geometries) == 0 {
		return referenceframe.NewSimpleModel(name), nil
	}
	cfg := &referenceframe.ModelConfigJSON{
		Name:  name,
		Links: []referenceframe.LinkConfig{},
	}
	parent := referenceframe.World
	for _, g := range geometries {
		f, err := referenceframe.NewStaticFrameWithGeometry(g.Label(), spatialmath.NewZeroPose(), g)
		if err != nil {
			return nil, err
		}
		lf, err := referenceframe.NewLinkConfig(f)
		if err != nil {
			return nil, err
		}
		lf.Parent = parent
		parent = g.Label()
		cfg.Links = append(cfg.Links, *lf)
	}
	return cfg.ParseConfig(name)
}
