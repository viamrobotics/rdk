// Package motion is the service that allows you to plan and execute movements.
package motion

import (
	"context"
	"time"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	servicepb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
)

func init() {
	resource.RegisterAPI(API, resource.APIRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterMotionServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.MotionService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
	})
}

// A Service controls the flow of moving components.
type Service interface {
	resource.Resource
	Move(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *servicepb.Constraints,
		extra map[string]interface{},
	) (bool, error)
	MoveOnMap(
		ctx context.Context,
		componentName resource.Name,
		destination spatialmath.Pose,
		slamName resource.Name,
		extra map[string]interface{},
	) (bool, error)
	MoveOnGlobe(
		ctx context.Context,
		componentName resource.Name,
		destination *geo.Point,
		heading float64,
		movementSensorName resource.Name,
		obstacles []*spatialmath.GeoObstacle,
		motionConfig *MotionConfiguration,
		extra map[string]interface{},
	) (bool, error)
	GetPose(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
	ListPlanStatuses(
		ctx context.Context,
		extra map[string]interface{},
	) ([]PlanStatus, error)
	GetPlan(
		ctx context.Context,
		r GetPlanRequest,
	) (OpIDPlans, error)
}

// GetPlanRequest describes the request to the GetPlan interface method.
// Contains the OperationID the returned plan(s) should be associated with
// and an Extra parameter.
type GetPlanRequest struct {
	OperationID uuid.UUID
	Extra       map[string]interface{}
}

// Step represents a single step of the plan
// Describes the pose each resource described by the plan
// should move to at that step.
type Step map[resource.Name]spatialmath.Pose

// Plan represnts a motion plan.
// Has a unique ID and a sequence of Steps
// which can be executed to follow the plan.
type Plan struct {
	ID    uuid.UUID
	Steps []Step
}

// PlanState denotes the state a Plan is in.
type PlanState = int32

const (
	// PlanStateUnspecified denotes an the Plan is in an unspecified state. This should never happen.
	PlanStateUnspecified = iota

	// PlanStateInProgress denotes an the Plan is in an in progress state. It is a temporary state.
	PlanStateInProgress

	// PlanStateCancelled denotes an the Plan is in a cancelled state. It is a terminal state.
	PlanStateCancelled

	// PlanStateSucceeded denotes an the Plan is in a succeeded state. It is a terminal state.
	PlanStateSucceeded

	// PlanStateFailed denotes an the Plan is in a failed state. It is a terminal state.
	PlanStateFailed
)

// PlanStatus represents a state change of a currently executing or previously executing Plan
// at a given point in time.
// Contains the PlanID, OperationID, State, Timestamp, and a Reason for the state change.
type PlanStatus struct {
	PlanID      uuid.UUID
	OperationID uuid.UUID
	State       PlanState
	Timestamp   time.Time
	Reason      string
}

// PlanWithStatus contains a plan, its current status, and all state changes that came prior
// sorted by ascending timestamp.
type PlanWithStatus struct {
	Plan          Plan
	Status        PlanStatus
	StatusHistory []PlanStatus
}

// OpIDPlans is the response of the GetPlan interface method.
// It contains the current PlanWithStatus & all prior plans
// associated with the OpID provided in the request.
type OpIDPlans struct {
	CurrentPlanWithPlanWithStatus PlanWithStatus
	ReplanHistory                 []PlanWithStatus
}

// MotionConfiguration specifies how to configure a call
//
//nolint:revive
type MotionConfiguration struct {
	VisionServices        []resource.Name
	PositionPollingFreqHz float64
	ObstaclePollingFreqHz float64
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
