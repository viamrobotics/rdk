// Package base defines the base that a robot uses to move around.
package base

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/base/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Base]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterBaseServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.BaseService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// SubtypeName is a constant that identifies the component resource API string "base".
const SubtypeName = "base"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Base's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// A Base represents a physical base of a robot.
type Base interface {
	resource.Resource
	resource.Actuator

	// MoveStraight moves the robot straight a given distance at a given speed.
	// If a distance or speed of zero is given, the base will stop.
	// This method blocks until completed or cancelled
	MoveStraight(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error

	// Spin spins the robot by a given angle in degrees at a given speed.
	// If a speed of 0 the base will stop.
	// This method blocks until completed or cancelled
	Spin(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error

	SetPower(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	// linear is in mmPerSec
	// angular is in degsPerSec
	SetVelocity(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error

	Properties(ctx context.Context, extra map[string]interface{}) (map[Feature]float64, error)
}

// A LocalBase represents a physical base of a robot that can report the width of itself.
type LocalBase interface {
	Base
	// Width returns the width of the base in millimeters.
	Width(ctx context.Context) (int, error)
}

// KinematicWrappable describes a base that can be wrapped with a kinematic model.
type KinematicWrappable interface {
	WrapWithKinematics(context.Context, slam.Service) (KinematicBase, error)
}

// KinematicBase is an interface for Bases that also satisfy the ModelFramer and InputEnabled interfaces.
type KinematicBase interface {
	Base
	referenceframe.ModelFramer
	referenceframe.InputEnabled
}

// FromDependencies is a helper for getting the named base from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Base, error) {
	return resource.FromDependencies[Base](deps, Named(name))
}

// FromRobot is a helper for getting the named base from the given Robot.
func FromRobot(r robot.Robot, name string) (Base, error) {
	return robot.ResourceFromRobot[Base](r, Named(name))
}

// NamesFromRobot is a helper for getting all base names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// CreateStatus creates a status from the base.
func CreateStatus(ctx context.Context, b Base) (*commonpb.ActuatorStatus, error) {
	isMoving, err := b.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &commonpb.ActuatorStatus{IsMoving: isMoving}, nil
}

// CollisionGeometry returns a spherical geometry that will encompass the base if it were to rotate the geometry specified in the config
// 360 degrees about the Z axis of the reference frame specified in the config.
func CollisionGeometry(cfg *referenceframe.LinkConfig) (spatialmath.Geometry, error) {
	// TODO(RSDK-1014): the orientation of this model will matter for collision checking,
	// and should match the convention of +Y being forward for bases
	if cfg == nil || cfg.Geometry == nil {
		return nil, errors.New("base not configured with a geometry on its frame, cannot create collision geometry for it")
	}
	geoCfg := cfg.Geometry
	r := geoCfg.TranslationOffset.Norm()
	switch geoCfg.Type {
	case spatialmath.BoxType:
		r += r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}.Norm() / 2
	case spatialmath.SphereType:
		r += geoCfg.R
	case spatialmath.CapsuleType:
		r += geoCfg.L / 2
	case spatialmath.UnknownType:
		// no type specified, iterate through supported types and try to infer intent
		if norm := (r3.Vector{X: geoCfg.X, Y: geoCfg.Y, Z: geoCfg.Z}).Norm(); norm > 0 {
			r += norm / 2
		} else if geoCfg.L != 0 {
			r += geoCfg.L / 2
		} else {
			r += geoCfg.R
		}
	case spatialmath.PointType:
	default:
		return nil, spatialmath.ErrGeometryTypeUnsupported
	}
	sphere, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), r, geoCfg.Label)
	if err != nil {
		return nil, err
	}
	return sphere, nil
}

// A Move describes instructions for a robot to spin followed by moving straight.
type Move struct {
	DistanceMm int
	MmPerSec   float64
	AngleDeg   float64
	DegsPerSec float64
	Extra      map[string]interface{}
}

// DoMove performs the given move on the given base.
func DoMove(ctx context.Context, move Move, base Base) error {
	if move.AngleDeg != 0 {
		err := base.Spin(ctx, move.AngleDeg, move.DegsPerSec, move.Extra)
		if err != nil {
			return err
		}
	}

	if move.DistanceMm != 0 {
		err := base.MoveStraight(ctx, move.DistanceMm, move.MmPerSec, move.Extra)
		if err != nil {
			return err
		}
	}

	return nil
}
