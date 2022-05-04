// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"math"
	"sync"

	"github.com/edaniels/golog"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/data"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/arm/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		Reconfigurable: WrapWithReconfigurable,
		Status: func(ctx context.Context, resource interface{}) (interface{}, error) {
			return CreateStatus(ctx, resource)
		},
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.ArmService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterArmServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})

	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: getEndPosition.String(),
	}, newGetEndPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    SubtypeName,
		MethodName: getJointPositions.String(),
	}, newGetJointPositionsCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "arm".
const SubtypeName = resource.SubtypeName("arm")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Arm's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// An Arm represents a physical robotic arm that exists in three-dimensional space.
type Arm interface {
	// GetEndPosition returns the current position of the arm.
	GetEndPosition(ctx context.Context) (*commonpb.Pose, error)

	// MoveToPosition moves the arm to the given absolute position.
	// The worldState argument should be treated as optional by all implementing drivers
	MoveToPosition(ctx context.Context, pose *commonpb.Pose, worldState *commonpb.WorldState) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions) error

	// GetJointPositions returns the current joint positions of the arm.
	GetJointPositions(ctx context.Context) (*pb.JointPositions, error)

	generic.Generic
	referenceframe.ModelFramer
	referenceframe.InputEnabled
}

var (
	_ = Arm(&reconfigurableArm{})
	_ = resource.Reconfigurable(&reconfigurableArm{})
)

// FromRobot is a helper for getting the named Arm from the given Robot.
func FromRobot(r robot.Robot, name string) (Arm, error) {
	res, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	part, ok := res.(Arm)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Arm", res)
	}
	return part, nil
}

// NamesFromRobot is a helper for getting all arm names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the arm.
func CreateStatus(ctx context.Context, resource interface{}) (*pb.Status, error) {
	arm, ok := resource.(Arm)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Arm", resource)
	}
	endPosition, err := arm.GetEndPosition(ctx)
	if err != nil {
		return nil, err
	}
	jointPositions, err := arm.GetJointPositions(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.Status{EndPosition: endPosition, JointPositions: jointPositions}, nil
}

type reconfigurableArm struct {
	mu     sync.RWMutex
	actual Arm
}

func (r *reconfigurableArm) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Do(ctx, cmd)
}

func (r *reconfigurableArm) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableArm) GetEndPosition(ctx context.Context) (*commonpb.Pose, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetEndPosition(ctx)
}

func (r *reconfigurableArm) MoveToPosition(ctx context.Context, pose *commonpb.Pose, worldState *commonpb.WorldState) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToPosition(ctx, pose, worldState)
}

func (r *reconfigurableArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToJointPositions(ctx, positionDegs)
}

func (r *reconfigurableArm) GetJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GetJointPositions(ctx)
}

func (r *reconfigurableArm) ModelFrame() referenceframe.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ModelFrame()
}

func (r *reconfigurableArm) CurrentInputs(ctx context.Context) ([]referenceframe.Input, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.CurrentInputs(ctx)
}

func (r *reconfigurableArm) GoToInputs(ctx context.Context, goal []referenceframe.Input) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.GoToInputs(ctx, goal)
}

func (r *reconfigurableArm) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableArm) Reconfigure(ctx context.Context, newArm resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newArm.(*reconfigurableArm)
	if !ok {
		return utils.NewUnexpectedTypeError(r, newArm)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// ShouldUpdate helps hinting the reconfiguration process on what strategy to use given a modified config.
// See robot.ShouldUpdateAction for more information.
func (r *reconfigurableArm) ShouldUpdate(c *config.Component) robot.ShouldUpdateAction {
	obj, canUpdate := r.actual.(interface {
		ShouldUpdate(config *config.Component) robot.ShouldUpdateAction
	})
	if canUpdate {
		return obj.ShouldUpdate(c)
	}
	return robot.Reconfigure
}

// WrapWithReconfigurable converts a regular Arm implementation to a reconfigurableArm.
// If arm is already a reconfigurableArm, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	arm, ok := r.(Arm)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("Arm", r)
	}
	if reconfigurable, ok := arm.(*reconfigurableArm); ok {
		return reconfigurable, nil
	}
	return &reconfigurableArm{actual: arm}, nil
}

// NewPositionFromMetersAndOV returns a three-dimensional arm position
// defined by a point in space in meters and an orientation defined as an OrientationVector.
// See robot.proto for a math explanation.
func NewPositionFromMetersAndOV(x, y, z, th, ox, oy, oz float64) *commonpb.Pose {
	return &commonpb.Pose{
		X:     x * 1000,
		Y:     y * 1000,
		Z:     z * 1000,
		OX:    ox,
		OY:    oy,
		OZ:    oz,
		Theta: th,
	}
}

// PositionGridDiff returns the euclidean distance between
// two arm positions in millimeters.
func PositionGridDiff(a, b *commonpb.Pose) float64 {
	diff := utils.Square(a.X-b.X) +
		utils.Square(a.Y-b.Y) +
		utils.Square(a.Z-b.Z)

	// Pythagorean theorum in 3d uses sqrt, not cube root
	// https://www.mathsisfun.com/geometry/pythagoras-3d.html
	return math.Sqrt(diff)
}

// PositionRotationDiff returns the sum of the squared differences between the angle axis components of two positions.
func PositionRotationDiff(a, b *commonpb.Pose) float64 {
	return utils.Square(a.Theta-b.Theta) +
		utils.Square(a.OX-b.OX) +
		utils.Square(a.OY-b.OY) +
		utils.Square(a.OZ-b.OZ)
}

// GoToWaypoints will visit in turn each of the joint position waypoints generated by a motion planner.
func GoToWaypoints(ctx context.Context, a Arm, waypoints [][]referenceframe.Input) error {
	for _, waypoint := range waypoints {
		err := ctx.Err() // make sure we haven't been cancelled
		if err != nil {
			return err
		}

		err = a.GoToInputs(ctx, waypoint)
		if err != nil {
			return err
		}
	}
	return nil
}
