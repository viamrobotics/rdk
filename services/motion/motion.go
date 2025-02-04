// Package motion is the service that allows you to plan and execute movements.
// For more information, see the [motion service docs].
//
// [motion service docs]: https://docs.viam.com/services/motion/
package motion

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	pb "go.viam.com/api/service/motion/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/motionplan"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           pb.RegisterMotionServiceHandlerFromEndpoint,
		RPCServiceDesc:              &pb.MotionService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// PlanHistoryReq describes the request to PlanHistory().
type PlanHistoryReq struct {
	// ComponentName the returned plans should be associated with.
	ComponentName resource.Name
	// When true, only the most recent plan will be returned which matches the ComponentName & ExecutionID if one was provided.
	LastPlanOnly bool
	// Optional, when not uuid.Nil it specifies the ExecutionID of the plans that should be returned.
	// Can be used to query plans from executions before the most recent one.
	ExecutionID ExecutionID
	Extra       map[string]interface{}
}

// MoveReq describes the request to the Move interface method.
type MoveReq struct {
	// ComponentName of the component to move
	ComponentName resource.Name
	// Goal destination the component should be moved to
	Destination *referenceframe.PoseInFrame
	// The external environment to be considered for the duration of the move
	WorldState *referenceframe.WorldState
	// Constraints which need to be satisfied during the movement
	Constraints *motionplan.Constraints
	Extra       map[string]interface{}
}

// MoveOnGlobeReq describes the request to the MoveOnGlobe interface method.
type MoveOnGlobeReq struct {
	// ComponentName of the component to move
	ComponentName resource.Name
	// Goal destination the component should be moved to
	Destination *geo.Point
	// Heading the component should have a when it reaches the goal.
	// Range [0-360] Left Hand Rule (N: 0, E: 90, S: 180, W: 270)
	Heading float64
	// Name of the momement sensor which can be used to derive Position & Heading
	MovementSensorName resource.Name
	// Static obstacles that should be navigated around
	Obstacles []*spatialmath.GeoGeometry
	// Set of bounds which the robot must remain within while navigating
	BoundingRegions []*spatialmath.GeoGeometry
	// Optional motion configuration
	MotionCfg *MotionConfiguration
	Extra     map[string]interface{}
}

func (r MoveOnGlobeReq) String() string {
	template := "motion.MoveOnGlobeReq{ComponentName: %s, " +
		"Destination: %+v, Heading: %f, MovementSensorName: %s, " +
		"Obstacles: %v, BoundingRegions: %v, MotionCfg: %#v, Extra: %s}"
	return fmt.Sprintf(template,
		r.ComponentName,
		r.Destination,
		r.Heading,
		r.MovementSensorName,
		r.Obstacles,
		r.BoundingRegions,
		r.MotionCfg,
		r.Extra)
}

// MoveOnMapReq describes a request to MoveOnMap.
type MoveOnMapReq struct {
	ComponentName resource.Name
	Destination   spatialmath.Pose
	SlamName      resource.Name
	MotionCfg     *MotionConfiguration
	Obstacles     []spatialmath.Geometry
	Extra         map[string]interface{}
}

func (r MoveOnMapReq) String() string {
	return fmt.Sprintf(
		"motion.MoveOnMapReq{ComponentName: %s, SlamName: %s, Destination: %+v, "+
			"MotionCfg: %#v, Obstacles: %s, Extra: %s}",
		r.ComponentName,
		r.SlamName,
		spatialmath.PoseToProtobuf(r.Destination),
		r.MotionCfg,
		r.Obstacles,
		r.Extra)
}

// StopPlanReq describes the request to StopPlan().
type StopPlanReq struct {
	// ComponentName of the plan which should be stopped
	ComponentName resource.Name
	Extra         map[string]interface{}
}

// ListPlanStatusesReq describes the request to ListPlanStatuses().
type ListPlanStatusesReq struct {
	// If true then only active plans will be returned.
	OnlyActivePlans bool
	Extra           map[string]interface{}
}

// PlanWithMetadata represents a motion plan with additional metadata used by the motion service.
type PlanWithMetadata struct {
	// Unique ID of the plan
	ID PlanID
	// Name of the component the plan is planning for
	ComponentName resource.Name
	// Unique ID of the execution
	ExecutionID ExecutionID
	// The motionplan itself
	motionplan.Plan
	// The GPS point to anchor visualized plans at
	AnchorGeoPose *spatialmath.GeoPose
}

// PlanState denotes the state a Plan is in.
type PlanState uint8

const (
	// PlanStateUnspecified denotes an the Plan is in an unspecified state. This should never happen.
	PlanStateUnspecified = iota

	// PlanStateInProgress denotes an the Plan is in an in progress state. It is a temporary state.
	PlanStateInProgress

	// PlanStateStopped denotes an the Plan is in a stopped state. It is a terminal state.
	PlanStateStopped

	// PlanStateSucceeded denotes an the Plan is in a succeeded state. It is a terminal state.
	PlanStateSucceeded

	// PlanStateFailed denotes an the Plan is in a failed state. It is a terminal state.
	PlanStateFailed
)

// TerminalStateSet is a set that defines the PlanState values which are terminal
// i.e. which represent the end of a plan.
var TerminalStateSet = map[PlanState]struct{}{
	PlanStateStopped:   {},
	PlanStateSucceeded: {},
	PlanStateFailed:    {},
}

// PlanID uniquely identifies a Plan.
type PlanID = uuid.UUID

// ExecutionID uniquely identifies an execution.
type ExecutionID = uuid.UUID

// PlanStatusWithID describes the state of a given plan at a
// point in time plus the PlanId, ComponentName and ExecutionID
// the status is associated with.
type PlanStatusWithID struct {
	PlanID        PlanID
	ComponentName resource.Name
	ExecutionID   ExecutionID
	Status        PlanStatus
}

// PlanStatus describes the state of a given plan at a
// point in time allong with an optional reason why the PlanStatus
// transitioned to that state.
type PlanStatus struct {
	State     PlanState
	Timestamp time.Time
	Reason    *string
}

// PlanWithStatus contains a plan, its current status, and all state changes that came prior
// sorted by ascending timestamp.
type PlanWithStatus struct {
	Plan          PlanWithMetadata
	StatusHistory []PlanStatus
}

// A Service controls the flow of moving components.
// For more information, see the [motion service docs].
//
// Move example:
//
//	motionService, err := motion.FromRobot(machine, "builtin")
//
//	// Assumes a gripper configured with name "my_gripper" on the machine
//	gripperName := gripper.Named("my_gripper")
//
//	// Define a destination Pose
//	destination := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: 0.1, Y: 0.0, Z: 0.0}))
//
//	// Create obstacles
//	boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.0, Y: 0.0, Z: 0.0})
//	boxDims := r3.Vector{X: 0.2, Y: 0.2, Z: 0.2} // 20cm x 20cm x 20cm box
//	obstacle, _ := spatialmath.NewBox(boxPose, boxDims, "obstacle_1")
//
//	geometryInFrame := referenceframe.NewGeometriesInFrame("base", []spatialmath.Geometry{obstacle})
//	obstacles := []*referenceframe.GeometriesInFrame{geometryInFrame}
//
//	// Create transforms
//	transform := referenceframe.NewLinkInFrame("gripper",
//		spatialmath.NewPoseFromPoint(r3.Vector{X: 0.1, Y: 0.0, Z: 0.1}), "transform_1", nil
//	)
//	transforms := []*referenceframe.LinkInFrame{transform}
//
//	// Create WorldState
//	worldState, err := referenceframe.NewWorldState(obstacles, transforms)
//
//	// Move gripper component
//
//	moved, err := motionService.Move(context.Background(), motion.MoveReq{
//		ComponentName: gripperName,
//		Destination: destination,
//		WorldState: WorldState
//	})
//
// For more information, see the [Move method docs].
//
// MoveOnMap example:
//
//	// Assumes a base with the name "my_base" is configured on the machine
//	myBaseResourceName := base.Named("my_base")
//	mySLAMServiceResourceName := slam.Named("my_slam_service")
//
//	// Define a destination Pose
//	myPose := spatialmath.NewPoseFromPoint(r3.Vector{Y: 10})
//
//	// Move the base component to the destination pose
//	executionID, err := motionService.MoveOnMap(context.Background(), motion.MoveOnMapReq{
//		ComponentName: myBaseResourceName,
//		Destination:   myPose,
//		SlamName:      mySLAMServiceResourceName,
//	})
//
//	// MoveOnMap is a non-blocking method and this line can optionally be added to block until the movement is done
//	err = motion.PollHistoryUntilSuccessOrError(
//		context.Background(),
//		motionService,
//		time.Duration(time.Second),
//		motion.PlanHistoryReq{
//			ComponentName: myBaseResourceName,
//			ExecutionID:   executionID,
//		},
//	)
//
// For more information, see the [MoveOnMap method docs].
//
// MoveOnGlobe example:
//
//	// Assumes a base with the name "myBase" is configured on the machine
//	// Get the resource names of the base and movement sensor
//	myBaseResourceName := base.Named("myBase")
//	myMvmntSensorResourceName := movementsensor.Named("my_movement_sensor")
//
//	// Define a destination Point at the GPS coordinates [0, 0]
//	myDestination := geo.NewPoint(0, 0)
//
//	// Move the base component to the designated geographic location, as reported by the movement sensor
//	executionID, err := motionService.MoveOnGlobe(context.Background(), motion.MoveOnGlobeReq{
//		ComponentName:      myBaseResourceName,
//		Destination:        myDestination,
//		MovementSensorName: myMvmntSensorResourceName,
//	})
//
//	// Assumes there is an active MoveOnMap() or MoveonGlobe() in progress for myBase
//	//	MoveOnGlobe is a non-blocking method and this line can optionally be added to block until the movement is done
//	err = motion.PollHistoryUntilSuccessOrError(
//		context.Background(),
//		motionService,
//		time.Duration(time.Second),
//		motion.PlanHistoryReq{
//			ComponentName: myBaseResourceName,
//			ExecutionID:   executionID,
//		},
//	)
//
// For more information, see the [MoveOnGlobe method docs].
//
// GetPose example:
//
//	// Insert code to connect to your machine.
//	// (see CONNECT tab of your machine's page in the Viam app)
//
//	// Assumes a gripper configured with name "my_gripper" on the machine
//	gripperName := gripper.Named("my_gripper")
//
//	// Access the motion service
//	motionService, err := motion.FromRobot(machine, "builtin")
//	if err != nil {
//	  logger.Fatal(err)
//	}
//
//	myArmMotionPose, err := motionService.GetPose(context.Background(), my_gripper, referenceframe.World, nil, nil)
//	if err != nil {
//	  logger.Fatal(err)
//	}
//	logger.Info("Position of myArm from the motion service:", myArmMotionPose.Pose().Point())
//	logger.Info("Orientation of myArm from the motion service:", myArmMotionPose.Pose().Orientation())
//
// For more information, see the [GetPose method docs].
//
// StopPlan example:
//
//	motionService, err := motion.FromRobot(machine, "builtin")
//	myBaseResourceName := base.Named("myBase")
//
//	myMvmntSensorResourceName := movement_sensor.Named("my_movement_sensor")
//	myDestination := geo.NewPoint(0, 0)
//
//	// Assuming a `MoveOnGlobe()`` started the execution
//	// Stop the base component which was instructed to move by `MoveOnGlobe()` or `MoveOnMap()`
//	err := motionService.StopPlan(context.Background(), motion.StopPlanReq{
//	    ComponentName: s.req.ComponentName,
//	})
//
// For more information, see the [StopPlan method docs].
//
// ListPlanStatuses example:
//
//	motionService, err := motion.FromRobot(machine, "builtin")
//
//	// Get the plan(s) of the base component's most recent execution i.e. `MoveOnGlobe()` or `MoveOnMap()` call.
//	planStatuses, err := motionService.ListPlanStatuses(context.Background(), motion.ListPlanStatusesReq{})
//
// For more information, see the [ListPlanStatuses method docs].
//
// PlanHistory example:
//
//	// Get the resource name of the base component
//	myBaseResourceName := base.Named("myBase")
//
//	// Get the plan history of the base component's most recent execution (e.g., MoveOnGlobe or MoveOnMap call)
//	planHistory, err := motionService.PlanHistory(context.Background(), motion.PlanHistoryReq{
//		ComponentName: myBaseResourceName,
//	})
//
// For more information, see the [PlanHistory method docs].
//
// [motion service docs]: https://docs.viam.com/operate/reference/services/motion/
// [Move method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#move
// [MoveOnMap method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#moveonmap
// [MoveOnGlobe method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#moveonglobe
// [GetPose method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#getpose
// [StopPlan method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#stopplan
// [ListPlanStatuses method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#listplanstatuses
// [PlanHistory method docs]: https://docs.viam.com/dev/reference/apis/services/motion/#getplan
type Service interface {
	resource.Resource

	// Move is the primary method to move multiple components or any object to a specified location.
	// Given a destination pose and a component, Move constructs a kinematic chain from goal to destination,
	// solves it while adhering to constraints, and executes the movement to avoid collisions with the machine itself
	// and other known objects. The above arguments are all grouped together in the MoveReq struct.
	Move(ctx context.Context, req MoveReq) (bool, error)

	// MoveOnMap moves a base component to a destination Pose on a SLAM map and returns a unique ExecutionID.
	// If the machine is already within PlanDeviationM of the goal, an error is returned.
	// Monitor progress with `GetPlan()` and `ListPlanStatuses()`, and check the machine's position via the SLAM service.
	// Designed for autonomous indoor navigation of rover bases.
	MoveOnMap(ctx context.Context, req MoveOnMapReq) (ExecutionID, error)

	// MoveOnGlobe moves a base component to a destination GPS point(latitude, longitude and returns a unique ExecutionID.
	// If the machine is already within PlanDeviationM of the goal, an error is returned.
	// This non-blocking method uses a movement sensor to verify the location of the base.
	// You can monitor progress with `GetPlan()` and `ListPlanStatuses()`. Designed for autonomous GPS navigation of rover bases.
	MoveOnGlobe(ctx context.Context, req MoveOnGlobeReq) (ExecutionID, error)

	// GetPose returns the location and orientation of a component within a frame system.
	// It returns a `PoseInFrame` describing the pose of the specified component relative to the specified destination frame.
	// The `supplemental_transforms` argument can be used to augment the machine's existing frame system with additional frames.
	GetPose(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)

	// StopPlan stops a base component being moved by an in progress `MoveOnGlobe()` or `MoveOnMap()` call.
	StopPlan(ctx context.Context, req StopPlanReq) error

	// ListPlanStatuses returns the statuses of plans created by `MoveOnGlobe()` or `MoveOnMap()` since the motion service initialized.
	// It includes plans that are in progress or have changed state in the last 24 hours.
	// All repeated fields are in chronological order.
	ListPlanStatuses(ctx context.Context, req ListPlanStatusesReq) ([]PlanStatusWithID, error)

	// PlanHistory returns the plan history of the most recent `MoveOnGlobe()` or `MoveOnMap()` call by default.
	// The history for earlier executions can be requested by providing an ExecutionID.
	// It returns a result if the execution is active or has changed state in the last 24 hours and the machine has not reinitialized.
	// Plans never change; replans always create new plans and replans share the ExecutionID of the previously executing plan.
	PlanHistory(ctx context.Context, req PlanHistoryReq) ([]PlanWithStatus, error)
}

// ObstacleDetectorName pairs a vision service name with a camera name.
type ObstacleDetectorName struct {
	VisionServiceName resource.Name
	CameraName        resource.Name
}

// MotionConfiguration specifies how to configure a call.
//
//nolint:revive
type MotionConfiguration struct {
	ObstacleDetectors     []ObstacleDetectorName
	PositionPollingFreqHz *float64
	ObstaclePollingFreqHz *float64
	PlanDeviationMM       float64
	LinearMPerSec         float64
	AngularDegsPerSec     float64
}

// SubtypeName is the name of the type of service.
const SubtypeName = "motion"

// API is a variable that identifies the motion service resource API.
var API = resource.APINamespaceRDK.WithServiceType(SubtypeName)

// Named is a helper for getting the named motion service's typed resource name.
func Named(name string) resource.Name {
	return resource.NewName(API, name)
}

// FromRobot is a helper for getting the named motion service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FromDependencies is a helper for getting the named motion service from a collection of dependencies.
func FromDependencies(deps resource.Dependencies, name string) (Service, error) {
	return resource.FromDependencies[Service](deps, Named(name))
}

// ToProto converts a PlanWithStatus to a *pb.PlanWithStatus.
func (pws PlanWithStatus) ToProto() *pb.PlanWithStatus {
	statusHistory := []*pb.PlanStatus{}
	for _, ps := range pws.StatusHistory {
		statusHistory = append(statusHistory, ps.ToProto())
	}

	planWithStatusPB := &pb.PlanWithStatus{
		Plan: pws.Plan.ToProto(),
	}

	if len(pws.StatusHistory) == 0 {
		return planWithStatusPB
	}

	planWithStatusPB.Status = statusHistory[0]
	planWithStatusPB.StatusHistory = statusHistory[1:]
	return planWithStatusPB
}

// ToProto converts a PlanStatusWithID to a *pb.PlanStatusWithID.
func (ps PlanStatusWithID) ToProto() *pb.PlanStatusWithID {
	return &pb.PlanStatusWithID{
		PlanId:        ps.PlanID.String(),
		ComponentName: rprotoutils.ResourceNameToProto(ps.ComponentName),
		ExecutionId:   ps.ExecutionID.String(),
		Status:        ps.Status.ToProto(),
	}
}

// ToProto converts a PlanStatus to a *pb.PlanStatus.
func (ps PlanStatus) ToProto() *pb.PlanStatus {
	return &pb.PlanStatus{
		State:     ps.State.ToProto(),
		Timestamp: timestamppb.New(ps.Timestamp),
		Reason:    ps.Reason,
	}
}

// ToProto converts a Plan to a *pb.Plan.
func (p PlanWithMetadata) ToProto() *pb.Plan {
	steps := []*pb.PlanStep{}
	if p.Plan != nil {
		for _, s := range p.Path() {
			steps = append(steps, motionplan.FrameSystemPosesToProto(s))
		}
	}

	return &pb.Plan{
		Id:            p.ID.String(),
		ComponentName: rprotoutils.ResourceNameToProto(p.ComponentName),
		ExecutionId:   p.ExecutionID.String(),
		Steps:         steps,
	}
}

// Renderable returns a copy of the struct substituting its Plan for a GeoPlan consisting of smuggled global coordinates
// This will only be done if the AnchorGeoPose field is non-nil, otherwise the original struct will be returned.
func (p PlanWithMetadata) Renderable() PlanWithMetadata {
	if p.AnchorGeoPose == nil {
		return p
	}
	return PlanWithMetadata{
		ID:            p.ID,
		ComponentName: p.ComponentName,
		ExecutionID:   p.ExecutionID,
		Plan:          motionplan.NewGeoPlan(p.Plan, p.AnchorGeoPose.Location()),
	}
}

// ToProto converts a PlanState to a pb.PlanState.
func (ps PlanState) ToProto() pb.PlanState {
	switch ps {
	case PlanStateInProgress:
		return pb.PlanState_PLAN_STATE_IN_PROGRESS
	case PlanStateStopped:
		return pb.PlanState_PLAN_STATE_STOPPED
	case PlanStateSucceeded:
		return pb.PlanState_PLAN_STATE_SUCCEEDED
	case PlanStateFailed:
		return pb.PlanState_PLAN_STATE_FAILED
	default:
		return pb.PlanState_PLAN_STATE_UNSPECIFIED
	}
}

func (ps PlanState) String() string {
	switch ps {
	case PlanStateInProgress:
		return "in progress"
	case PlanStateStopped:
		return "stopped"
	case PlanStateSucceeded:
		return "succeeded"
	case PlanStateFailed:
		return "failed"
	case PlanStateUnspecified:
		return "unspecified"
	default:
		return "unknown"
	}
}
