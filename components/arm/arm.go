// Package arm defines the arm that a robot uses to manipulate objects.
package arm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/arm/v1"
	motionpb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Arm]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterArmServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.ArmService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})

	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: endPosition.String(),
	}, newEndPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		Subtype:    Subtype,
		MethodName: jointPositions.String(),
	}, newJointPositionsCollector)
}

// SubtypeName is a constant that identifies the component resource subtype string "arm".
const (
	SubtypeName = resource.SubtypeName("arm")
)

// MTPoob is a string that all MoveToPosition errors should contain if the method is called
// and there are joints which are out of bounds.
const MTPoob = "cartesian movements are not allowed when arm joints are out of bounds"

var (
	defaultLinearConstraint  = &motionpb.LinearConstraint{}
	defaultArmPlannerOptions = &motionpb.Constraints{
		LinearConstraint: []*motionpb.LinearConstraint{defaultLinearConstraint},
	}
)

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
	resource.Resource
	resource.Actuator
	referenceframe.ModelFramer
	referenceframe.InputEnabled

	// EndPosition returns the current position of the arm.
	EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)

	// MoveToPosition moves the arm to the given absolute position.
	// This will block until done or a new operation cancels this one
	MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	// This will block until done or a new operation cancels this one
	MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error

	// JointPositions returns the current joint positions of the arm.
	JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error)
}

// ErrStopUnimplemented is used for when Stop is unimplemented.
var ErrStopUnimplemented = errors.New("Stop unimplemented")

// FromDependencies is a helper for getting the named arm from a collection of
// dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Arm, error) {
	return resource.FromDependencies[Arm](deps, Named(name))
}

// FromRobot is a helper for getting the named Arm from the given Robot.
func FromRobot(r robot.Robot, name string) (Arm, error) {
	return robot.ResourceFromRobot[Arm](r, Named(name))
}

// NamesFromRobot is a helper for getting all arm names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesBySubtype(r, Subtype)
}

// CreateStatus creates a status from the arm.
func CreateStatus(ctx context.Context, a Arm) (*pb.Status, error) {
	jointPositions, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	model := a.ModelFrame()
	endPosition, err := motionplan.ComputePosition(model, jointPositions)
	if err != nil {
		return nil, err
	}
	isMoving, err := a.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{EndPosition: spatialmath.PoseToProtobuf(endPosition), JointPositions: jointPositions, IsMoving: isMoving}, nil
}

// Move is a helper function to abstract away movement for general arms.
func Move(ctx context.Context, logger golog.Logger, a Arm, dst spatialmath.Pose) error {
	joints, err := a.JointPositions(ctx, nil)
	if err != nil {
		return err
	}
	model := a.ModelFrame()
	// check that joint positions are not out of bounds
	_, err = motionplan.ComputePosition(model, joints)
	if err != nil && strings.Contains(err.Error(), referenceframe.OOBErrString) {
		return errors.New(MTPoob + ": " + err.Error())
	} else if err != nil {
		return err
	}

	solution, err := Plan(ctx, logger, a, dst)
	if err != nil {
		return err
	}
	return GoToWaypoints(ctx, a, solution)
}

// Plan is a helper function to be called by arm implementations to abstract away the default procedure for using the
// motion planning library with arms.
func Plan(ctx context.Context, logger golog.Logger, a Arm, dst spatialmath.Pose) ([][]referenceframe.Input, error) {
	armFrame := a.ModelFrame()
	jp, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	return motionplan.PlanFrameMotion(ctx, logger, dst, armFrame, armFrame.InputFromProtobuf(jp), defaultArmPlannerOptions, nil)
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

// CheckDesiredJointPositions validates that the desired joint positions either bring the joint back
// in bounds or do not move the joint more out of bounds.
func CheckDesiredJointPositions(ctx context.Context, a Arm, desiredJoints []float64) error {
	currentJointPos, err := a.JointPositions(ctx, nil)
	if err != nil {
		return err
	}
	checkPositions := currentJointPos.Values
	model := a.ModelFrame()
	joints := model.ModelConfig().Joints
	for i, val := range desiredJoints {
		max := joints[i].Max
		min := joints[i].Min
		currPosition := checkPositions[i]
		// to make sure that val is a valid input
		// it must either bring the joint more
		// inbounds or keep the joint inbounds.
		if currPosition > max {
			max = currPosition
		} else if currPosition < min {
			min = currPosition
		}
		if val > max || val < min {
			return fmt.Errorf("joint %v needs to be within range [%v, %v] and cannot be moved to %v", i, min, max, val)
		}
	}
	return nil
}
