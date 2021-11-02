// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"math"
	"sync"

	"github.com/go-errors/errors"
	viamutils "go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "arm"
const SubtypeName = resource.SubtypeName("arm")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Arm's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

// An Arm represents a physical robotic arm that exists in three-dimensional space.
type Arm interface {

	// CurrentPosition returns the current position of the arm.
	CurrentPosition(ctx context.Context) (*pb.Pose, error)

	// MoveToPosition moves the arm to the given absolute position.
	MoveToPosition(ctx context.Context, c *pb.Pose) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error

	// CurrentJointPositions returns the current joint positions of the arm.
	CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error)

	// JointMoveDelta moves a specific joint of the arm by the given amount.
	JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error
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

func (r *reconfigurableArm) CurrentPosition(ctx context.Context) (*pb.Pose, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.CurrentPosition(ctx)
}

func (r *reconfigurableArm) MoveToPosition(ctx context.Context, c *pb.Pose) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToPosition(ctx, c)
}

func (r *reconfigurableArm) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.MoveToJointPositions(ctx, pos)
}

func (r *reconfigurableArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.CurrentJointPositions(ctx)
}

func (r *reconfigurableArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.JointMoveDelta(ctx, joint, amountDegs)
}

func (r *reconfigurableArm) Close() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(r.actual)
}

func (r *reconfigurableArm) Reconfigure(newArm resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newArm.(*reconfigurableArm)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", r, newArm)
	}
	if err := viamutils.TryClose(r.actual); err != nil {
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
// See robot.proto for a math explanation
func NewPositionFromMetersAndOV(x, y, z, th, ox, oy, oz float64) *pb.Pose {
	return &pb.Pose{
		X:     x * 1000,
		Y:     y * 1000,
		Z:     z * 1000,
		OX:    ox,
		OY:    oy,
		OZ:    oz,
		Theta: th,
	}
}

// JointPositionsToRadians converts the given positions into a slice
// of radians.
func JointPositionsToRadians(jp *pb.JointPositions) []float64 {
	n := make([]float64, len(jp.Degrees))
	for idx, d := range jp.Degrees {
		n[idx] = utils.DegToRad(d)
	}
	return n
}

// JointPositionsFromRadians converts the given slice of radians into
// joint positions (represented in degrees).
func JointPositionsFromRadians(radians []float64) *pb.JointPositions {
	n := make([]float64, len(radians))
	for idx, a := range radians {
		n[idx] = utils.RadToDeg(a)
	}
	return &pb.JointPositions{Degrees: n}
}

// PositionGridDiff returns the euclidean distance between
// two arm positions in millimeters.
func PositionGridDiff(a, b *pb.Pose) float64 {
	diff := utils.Square(a.X-b.X) +
		utils.Square(a.Y-b.Y) +
		utils.Square(a.Z-b.Z)

	// Pythagorean theorum in 3d uses sqrt, not cube root
	// https://www.mathsisfun.com/geometry/pythagoras-3d.html
	return math.Sqrt(diff)
}

// PositionRotationDiff returns the sum of the squared differences between the angle axis components of two positions
func PositionRotationDiff(a, b *pb.Pose) float64 {
	return utils.Square(a.Theta-b.Theta) +
		utils.Square(a.OX-b.OX) +
		utils.Square(a.OY-b.OY) +
		utils.Square(a.OZ-b.OZ)
}
