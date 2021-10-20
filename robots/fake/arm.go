package fake

import (
	"context"
	"math"

	"github.com/go-errors/errors"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func init() {
	registry.RegisterComponent(arm.Subtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArm(config)
		},
		Frame: func(name string) (referenceframe.Frame, error) {
			point := r3.Vector{500, 0, 300}
			pose := spatialmath.NewPoseFromAxisAngle(point, r3.Vector{0, 1, 0}, math.Pi/2.)
			return referenceframe.NewStaticFrame(name, pose)
		},
	})
}

// NewArm returns a new fake arm.
func NewArm(cfg config.Component) (arm.Arm, error) {
	name := cfg.Name
	point := r3.Vector{500, 0, 300}
	pose := spatialmath.NewPoseFromAxisAngle(point, r3.Vector{0, 1, 0}, math.Pi/2.)
	frame, err := referenceframe.NewStaticFrame(name, pose)
	if err != nil {
		return nil, err
	}
	return &Arm{
		Name:        name,
		position:    &pb.ArmPosition{},
		joints:      &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
		frame:       frame,
		frameconfig: cfg.Frame,
	}, nil
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	Name        string
	position    *pb.ArmPosition
	joints      *pb.JointPositions
	CloseCount  int
	frame       referenceframe.Frame
	frameconfig *config.Frame
}

// Frame returns the intrinsic frame of the arm
func (a *Arm) Frame() referenceframe.Frame {
	return a.frame
}

// FrameSystemLink returns all the information necessary for including the arm in a FrameSystem
func (a *Arm) FrameSystemLink() (*config.Frame, referenceframe.Frame) {
	return a.frameconfig, a.frame
}

// CurrentPosition returns the set position.
func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	return errors.New("arm JointMoveDelta does nothing")
}

// Close does nothing.
func (a *Arm) Close() error {
	a.CloseCount++
	return nil
}
