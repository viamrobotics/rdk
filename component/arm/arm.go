// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"fmt"
	"math"
	"sync"

	viamutils "go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	"go.viam.com/core/utils"
)

// ResourceSubtype is a constant that identifies the component resource subtype
const ResourceSubtype = "core:component:arm"

// An Arm represents a physical robotic arm that exists in three-dimensional space.
type Arm interface {

	// CurrentPosition returns the current position of the arm.
	CurrentPosition(ctx context.Context) (*pb.ArmPosition, error)

	// MoveToPosition moves the arm to the given absolute position.
	MoveToPosition(ctx context.Context, c *pb.ArmPosition) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error

	// CurrentJointPositions returns the current joint positions of the arm.
	CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error)

	// JointMoveDelta moves a specific joint of the arm by the given amount.
	JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error
}

var _ = Arm(&proxyArm{})
var _ = resource.Reconfigurable(&proxyArm{})

type proxyArm struct {
	mu     sync.RWMutex
	actual Arm
}

func (p *proxyArm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.CurrentPosition(ctx)
}

func (p *proxyArm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveToPosition(ctx, c)
}

func (p *proxyArm) MoveToJointPositions(ctx context.Context, pos *pb.JointPositions) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.MoveToJointPositions(ctx, pos)
}

func (p *proxyArm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.CurrentJointPositions(ctx)
}

func (p *proxyArm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.actual.JointMoveDelta(ctx, joint, amountDegs)
}

func (p *proxyArm) Close() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return viamutils.TryClose(p.actual)
}

func (p *proxyArm) Reconfigure(newArm resource.Reconfigurable) {
	p.mu.Lock()
	defer p.mu.Unlock()
	actual, ok := newArm.(*proxyArm)
	if !ok {
		panic(fmt.Errorf("expected new arm to be %T but got %T", p, newArm))
	}
	if err := viamutils.TryClose(p.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	p.actual = actual.actual
}

// ToProxyArm converts a regular Arm implementation to a proxyArm.
// If arm is already a proxyArm, then nothing is done.
func ToProxyArm(arm Arm) Arm {
	if proxy, ok := arm.(*proxyArm); ok {
		return proxy
	}
	return &proxyArm{actual: arm}
}

// NewPositionFromMetersAndOV returns a three-dimensional arm position
// defined by a point in space in meters and an orientation defined as an OrientationVector.
// See robot.proto for a math explanation
func NewPositionFromMetersAndOV(x, y, z, th, ox, oy, oz float64) *pb.ArmPosition {
	return &pb.ArmPosition{
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
func PositionGridDiff(a, b *pb.ArmPosition) float64 {
	diff := utils.Square(a.X-b.X) +
		utils.Square(a.Y-b.Y) +
		utils.Square(a.Z-b.Z)

	// Pythagorean theorum in 3d uses sqrt, not cube root
	// https://www.mathsisfun.com/geometry/pythagoras-3d.html
	return math.Sqrt(diff)
}

// PositionRotationDiff returns the sum of the squared differences between the angle axis components of two positions
func PositionRotationDiff(a, b *pb.ArmPosition) float64 {
	return utils.Square(a.Theta-b.Theta) +
		utils.Square(a.OX-b.OX) +
		utils.Square(a.OY-b.OY) +
		utils.Square(a.OZ-b.OZ)
}
