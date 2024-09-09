//go:build !no_cgo

// Package arm defines the arm that a robot uses to manipulate objects.
// For more information, see the [arm component docs].
//
// [arm component docs]: https://docs.viam.com/components/arm/
package arm

import (
	"context"
	"fmt"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Arm]{
		Status:                      resource.StatusFunc(CreateStatus),
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterArmServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.ArmService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})

	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: endPosition.String(),
	}, newEndPositionCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: jointPositions.String(),
	}, newJointPositionsCollector)
}

// SubtypeName is a constant that identifies the component resource API string "arm".
const SubtypeName = "arm"

// API is a variable that identifies the component resource API.
var API = resource.APINamespaceRDK.WithComponentType(SubtypeName)

// Named is a helper for getting the named Arm's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// An Arm represents a physical robotic arm that exists in three-dimensional space.
// For more information, see the [arm component docs].
//
// EndPosition example:
//
//	myArm, err := arm.FromRobot(machine, "my_arm")
//	// Get the end position of the arm as a Pose.
//	pos, err := myArm.EndPosition(context.Background(), nil)
//
// MoveToPosition example:
//
//	myArm, err := arm.FromRobot(machine, "my_arm")
//	// Create a Pose for the arm.
//	examplePose := spatialmath.NewPose(
//	        r3.Vector{X: 5, Y: 5, Z: 5},
//	        &spatialmath.OrientationVectorDegrees{0X: 5, 0Y: 5, Theta: 20}
//	)
//
//	// Move your arm to the Pose.
//	err = myArm.MoveToPosition(context.Background(), examplePose, nil)
//
// MoveToJointPositions example:
//
//	// Assumes you have imported "go.viam.com/api/component/arm/v1" as `componentpb`
//	myArm, err := arm.FromRobot(machine, "my_arm")
//
//	// Declare an array of values with your desired rotational value for each joint on the arm.
//	degrees := []float64{4.0, 5.0, 6.0}
//
//	// Declare a new JointPositions with these values.
//	jointPos := &componentpb.JointPositions{Values: degrees}
//
//	// Move each joint of the arm to the position these values specify.
//	err = myArm.MoveToJointPositions(context.Background(), jointPos, nil)
//
// JointPositions example:
//
//	myArm , err := arm.FromRobot(machine, "my_arm")
//
//	// Get the current position of each joint on the arm as JointPositions.
//	pos, err := myArm.JointPositions(context.Background(), nil)
//
// [arm component docs]: https://docs.viam.com/components/arm/
type Arm interface {
	resource.Resource
	referenceframe.ModelFramer
	resource.Shaped
	resource.Actuator
	referenceframe.InputEnabled

	// EndPosition returns the current position of the arm.
	EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)

	// MoveToPosition moves the arm to the given absolute position.
	// This will block until done or a new operation cancels this one.
	MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	// This will block until done or a new operation cancels this one.
	MoveToJointPositions(ctx context.Context, positionDegs *pb.JointPositions, extra map[string]interface{}) error

	// JointPositions returns the current joint positions of the arm.
	JointPositions(ctx context.Context, extra map[string]interface{}) (*pb.JointPositions, error)
}

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
	return robot.NamesByAPI(r, API)
}

// CreateStatus creates a status from the arm. This will report calculated end effector positions even if the given
// arm is perceived to be out of bounds.
func CreateStatus(ctx context.Context, a Arm) (*pb.Status, error) {
	var jointPositions *pb.JointPositions
	joints, err := a.JointPositions(ctx, nil)
	if err != nil {
		return nil, err
	}
	jointPositions = joints

	var endPosition *v1.Pose
	model := a.ModelFrame()
	if model != nil && joints != nil {
		if endPose, err := referenceframe.ComputeOOBPosition(model, model.InputFromProtobuf(jointPositions)); err == nil {
			endPosition = spatialmath.PoseToProtobuf(endPose)
		}
	}

	isMoving, err := a.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.Status{EndPosition: endPosition, JointPositions: jointPositions, IsMoving: isMoving}, nil
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
func CheckDesiredJointPositions(ctx context.Context, a Arm, desiredInputs []referenceframe.Input) error {
	currentJointPos, err := a.JointPositions(ctx, nil)
	if err != nil {
		return err
	}
	model := a.ModelFrame()
	checkPositions := model.InputFromProtobuf(currentJointPos)
	limits := model.DoF()
	for i, val := range desiredInputs {
		max := limits[i].Max
		min := limits[i].Min
		currPosition := checkPositions[i]
		// to make sure that val is a valid input
		// it must either bring the joint more
		// inbounds or keep the joint inbounds.
		if currPosition.Value > limits[i].Max {
			max = currPosition.Value
		} else if currPosition.Value < limits[i].Min {
			min = currPosition.Value
		}
		if val.Value > max || val.Value < min {
			return fmt.Errorf("joint %v needs to be within range [%v, %v] and cannot be moved to %v", i, min, max, val.Value)
		}
	}
	return nil
}
