// Package arm defines the arm that a robot uses to manipulate objects.
// For more information, see the [arm component docs].
//
// [arm component docs]: https://docs.viam.com/components/arm/
package arm

import (
	"context"
	"fmt"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Arm]{
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
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: doCommand.String(),
	}, newDoCommandCollector)
	data.RegisterCollector(data.MethodMetadata{
		API:        API,
		MethodName: getWorldPose.String(),
	}, newGetWorldPoseCollector)
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
//	myArm, err := arm.FromProvider(machine, "my_arm")
//	// Get the end position of the arm as a Pose.
//	pos, err := myArm.EndPosition(context.Background(), nil)
//
// For more information, see the [EndPosition method docs].
//
// MoveToPosition example:
//
//	myArm, err := arm.FromProvider(machine, "my_arm")
//	// Create a Pose for the arm.
//	examplePose := spatialmath.NewPose(
//	        r3.Vector{X: 5, Y: 5, Z: 5},
//	        &spatialmath.OrientationVectorDegrees{OX: 5, OY: 5, Theta: 20},
//	)
//
//	// Move your arm to the Pose.
//	err = myArm.MoveToPosition(context.Background(), examplePose, nil)
//
// For more information, see the [MoveToPosition method docs].
//
// MoveToJointPositions example:
//
//	myArm, err := arm.FromProvider(machine, "my_arm")
//
//	// Declare an array of values with your desired rotational value (in radians) for each joint on the arm.
//	inputs := referenceframe.FloatsToInputs([]float64{0, math.Pi/2, math.Pi})
//
//	// Move each joint of the arm to the positions specified in the above slice
//	err = myArm.MoveToJointPositions(context.Background(), inputs, nil)
//
// For more information, see the [MoveToJointPositions method docs].
//
// MoveThroughJointPositions example:
//
//	myArm, err := arm.FromProvider(machine, "my_arm")
//
//	// Declare a 2D array of values with your desired rotational value (in radians) for each joint on the arm.
//	inputs := [][]referenceframe.Input{
//		referenceframe.FloatsToInputs([]float64{0, math.Pi/2, math.Pi})
//		referenceframe.FloatsToInputs([]float64{0, 0, 0})
//	}
//
//	// Move each joint of the arm through the positions in the slice defined above
//	err = myArm.MoveThroughJointPositions(context.Background(), inputs, nil, nil)
//
// For more information, see the [MoveThroughJointPositions method docs].
//
// JointPositions example:
//
//	myArm, err := arm.FromProvider(machine, "my_arm")
//
//	// Get the current position of each joint on the arm as JointPositions.
//	pos, err := myArm.JointPositions(context.Background(), nil)
//
// For more information, see the [JointPositions method docs].
//
// [arm component docs]: https://docs.viam.com/components/arm/
// [EndPosition method docs]: https://docs.viam.com/dev/reference/apis/components/arm/#getendposition
// [MoveToPosition method docs]: https://docs.viam.com/dev/reference/apis/components/arm/#movetoposition
// [MoveToJointPositions method docs]: https://docs.viam.com/dev/reference/apis/components/arm/#movetojointpositions
// [MoveThroughJointPositions method docs]: https://docs.viam.com/dev/reference/apis/components/arm/#movethroughjointpositions
// [JointPositions method docs]: https://docs.viam.com/dev/reference/apis/components/arm/#getjointpositions
type Arm interface {
	resource.Resource
	resource.Shaped
	resource.Actuator
	framesystem.InputEnabled

	// EndPosition returns the current position of the arm.
	EndPosition(ctx context.Context, extra map[string]interface{}) (spatialmath.Pose, error)

	// MoveToPosition moves the arm to the given absolute position.
	// This will block until done or a new operation cancels this one.
	MoveToPosition(ctx context.Context, pose spatialmath.Pose, extra map[string]interface{}) error

	// MoveToJointPositions moves the arm's joints to the given positions.
	// This will block until done or a new operation cancels this one.
	MoveToJointPositions(ctx context.Context, positions []referenceframe.Input, extra map[string]interface{}) error

	// MoveThroughJointPositions moves the arm's joints through the given positions in the order they are specified.
	// This will block until done or a new operation cancels this one.
	MoveThroughJointPositions(ctx context.Context, positions [][]referenceframe.Input, options *MoveOptions, extra map[string]any) error

	// MoveThroughJointPositionsStreamed receives a stream of trajectory point batches and executes
	// them in order.
	//
	// The framework feeds one slice per wire TrajectoryBatch from the gRPC request stream into
	// `batches` and closes the channel when the client signals end-of-stream. The implementation
	// writes one Response to `responses` per acknowledgment it wants to send the client; the
	// framework forwards each to the gRPC response stream. The implementation returns when it has
	// finished (nil) or detects a fault (an error, which is propagated to the client as the
	// terminal gRPC status).
	//
	// Iteration is the obvious nested-loop shape:
	//   for batch := range batches { for _, p := range batch { ... } }
	//
	// Channel ownership note: the framework owns `batches` (writes and closes it) and owns the
	// lifetime of `responses` (closes it after the implementation returns). The implementation
	// only reads from `batches` and only writes to `responses`; it must NOT close either. This
	// deviates from Go's usual sender-closes convention, but lets driver code be trivially
	// correct: write responses, return when done.
	MoveThroughJointPositionsStreamed(
		ctx context.Context,
		batches <-chan []TrajectoryPoint,
		responses chan<- Response,
		extra map[string]interface{},
	) error

	// JointPositions returns the current joint positions of the arm.
	JointPositions(ctx context.Context, extra map[string]interface{}) ([]referenceframe.Input, error)

	// Get3DModels returns the 3D models of the arm.
	Get3DModels(ctx context.Context, extra map[string]interface{}) (map[string]*commonpb.Mesh, error)
}

// TrajectoryPoint is one waypoint in a streamed joint-space trajectory.
type TrajectoryPoint struct {
	// Time from the start of the motion at which this waypoint should be reached. Must be zero
	// for the first point of the stream and strictly increasing thereafter.
	Time time.Duration

	// Joint positions at this waypoint.
	Positions []referenceframe.Input

	// Optional target kinematic constraints at this waypoint.
	Constraints *KinematicConstraints
}

// KinematicConstraints carries the optional target joint velocities (and optional
// joint accelerations) attached to a TrajectoryPoint.
type KinematicConstraints struct {
	// Target joint velocities at this waypoint.
	Velocities []float64

	// Optional target joint accelerations at this waypoint. nil if absent.
	Accelerations []float64
}

// Response is the per-acknowledgment payload an implementation may emit on
// `MoveThroughJointPositionsStreamed`'s response channel. The PoC carries no
// fields; future expansion (per-batch status, executed-up-to-time, etc.) lives
// here.
type Response struct{}

// Deprecated: FromDependencies is a helper for getting the named arm from a collection of
// dependencies. Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromDependencies(deps resource.Dependencies, name string) (Arm, error) {
	return resource.FromDependencies[Arm](deps, Named(name))
}

// Deprecated: FromRobot is a helper for getting the named Arm from the given Robot.
// Use FromProvider instead.
//
//nolint:revive // ignore exported comment check
func FromRobot(r robot.Robot, name string) (Arm, error) {
	return robot.ResourceFromRobot[Arm](r, Named(name))
}

// FromProvider is a helper for getting the named arm from a resource Provider (collection of Dependencies or a Robot).
func FromProvider(provider resource.Provider, name string) (Arm, error) {
	return resource.FromProvider[Arm](provider, Named(name))
}

// NamesFromRobot is a helper for getting all arm names from the given Robot.
func NamesFromRobot(r robot.Robot) []string {
	return robot.NamesByAPI(r, API)
}

// CheckDesiredJointPositions validates that the desired joint positions either bring the joint back
// in bounds or do not move the joint more out of bounds.
func CheckDesiredJointPositions(ctx context.Context, a Arm, desiredInputs []referenceframe.Input) error {
	currentJointPos, err := a.JointPositions(ctx, nil)
	if err != nil {
		return err
	}
	model, err := a.Kinematics(ctx)
	if err != nil {
		return err
	}
	limits := model.DoF()

	return checkDesiredJointPositions(limits, currentJointPos, desiredInputs)
}

func checkDesiredJointPositions(
	limits []referenceframe.Limit, currentJointPos, desiredInputs []referenceframe.Input,
) error {
	for i, val := range desiredInputs {
		//nolint: revive
		max := limits[i].Max
		//nolint: revive
		min := limits[i].Min
		currPosition := currentJointPos[i]
		// to make sure that val is a valid input it must either bring the joint closer inbounds or keep the joint inbounds.
		if currPosition > limits[i].Max {
			//nolint: revive
			max = currPosition
		} else if currPosition < limits[i].Min {
			//nolint: revive
			min = currPosition
		}
		if val > max || val < min {
			return fmt.Errorf("joint %v needs to be within range [%v, %v] and cannot be moved to %v",
				i,
				utils.RadToDeg(min),
				utils.RadToDeg(max),
				utils.RadToDeg(val),
			)
		}
	}
	return nil
}
