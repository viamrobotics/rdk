package fake

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/go-errors/errors"

	"go.viam.com/utils"

	"go.viam.com/core/arm"
	"go.viam.com/core/config"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	"go.viam.com/core/robot"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func init() {
	registry.RegisterComponentCreator(arm.ResourceSubtype, "fake", registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (resource.Resource, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewArm(config.Name), nil
		},
		Frame: func(name string) (referenceframe.Frame, error) {
			point := r3.Vector{500, 0, 300}
			pose := spatialmath.NewPoseFromAxisAngle(point, r3.Vector{0, 1, 0}, math.Pi/2.)
			return referenceframe.NewStaticFrame(name, pose)
		},
	})
}

// NewArm returns a new fake arm.
func NewArm(name string) *Arm {
	return &Arm{
		Name:     name,
		position: &pb.ArmPosition{},
		joints:   &pb.JointPositions{Degrees: []float64{0, 0, 0, 0, 0, 0}},
	}
}

// Arm is a fake arm that can simply read and set properties.
type Arm struct {
	mu         sync.RWMutex
	Name       string
	position   *pb.ArmPosition
	joints     *pb.JointPositions
	CloseCount int
}

// CurrentPosition returns the set position.
func (a *Arm) CurrentPosition(ctx context.Context) (*pb.ArmPosition, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.position, nil
}

// MoveToPosition sets the position.
func (a *Arm) MoveToPosition(ctx context.Context, c *pb.ArmPosition) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	a.position = c
	return nil
}

// MoveToJointPositions sets the joints.
func (a *Arm) MoveToJointPositions(ctx context.Context, joints *pb.JointPositions) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	a.joints = joints
	return nil
}

// CurrentJointPositions returns the set joints.
func (a *Arm) CurrentJointPositions(ctx context.Context) (*pb.JointPositions, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.joints, nil
}

// JointMoveDelta returns an error.
func (a *Arm) JointMoveDelta(ctx context.Context, joint int, amountDegs float64) error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return errors.New("arm JointMoveDelta does nothing")
}

// Close does nothing.
func (a *Arm) Close() error {
	a.CloseCount++
	return nil
}

// Reconfigure reconfigures the current resource to the resource passed in.
func (a *Arm) Reconfigure(newResource resource.Resource) {
	a.mu.Lock()
	defer a.mu.Unlock()
	actual, ok := newResource.(*Arm)
	if !ok {
		panic(fmt.Errorf("expected new resource to be %T but got %T", actual, newResource))
	}
	if err := utils.TryClose(a); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	a.Name = actual.Name
	a.position = actual.position
	a.joints = actual.joints
}
