// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"math"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/utils"
)

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
	MoveToPosition(ctx context.Context, pose *commonpb.Pose) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	MoveToJointPositions(ctx context.Context, positionDegs *pb.ArmJointPositions) error

	// GetJointPositions returns the current joint positions of the arm.
	GetJointPositions(ctx context.Context) (*pb.ArmJointPositions, error)

	referenceframe.ModelFramer
	referenceframe.InputEnabled
}

var (
	_ = Arm(&reconfigurableArm{})
	_ = resource.Reconfigurable(&reconfigurableArm{})
)

type reconfigurableArm struct {
	mu     sync.RWMutex
	actual Arm
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

func (r *reconfigurableArm) MoveToPosition(ctx context.Context, pose *commonpb.Pose) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToPosition(ctx, pose)
}

func (r *reconfigurableArm) MoveToJointPositions(ctx context.Context, positionDegs *pb.ArmJointPositions) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToJointPositions(ctx, positionDegs)
}

func (r *reconfigurableArm) GetJointPositions(ctx context.Context) (*pb.ArmJointPositions, error) {
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
		return errors.Errorf("expected new arm to be %T but got %T", r, newArm)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Arm implementation to a reconfigurableArm.
// If arm is already a reconfigurableArm, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	arm, ok := r.(Arm)
	if !ok {
		return nil, errors.Errorf("expected resource to be Arm but got %T", r)
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
		err := a.GoToInputs(ctx, waypoint)
		if err != nil {
			return err
		}
	}
	return nil
}
