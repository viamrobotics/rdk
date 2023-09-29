// Package motion is the service that allows you to plan and execute movements.
package motion

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/timestamppb"

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

// PlanHistoryReq describes the request to the PlanHistory interface method.
// Contains the ComponentName the returned plan(s) should be associated with,
// an optional ExecutionID and an Extra parameter.
// If LastPlanOnly is set to true then only the most recent plan for the
// component & execution in question is returned.
type PlanHistoryReq struct {
	ComponentName resource.Name
	LastPlanOnly  bool
	ExecutionID   uuid.UUID
	Extra         map[string]interface{}
}

// MoveOnGlobeReq describes the request to the GetPlan interface method.
// Contains the ComponentName the returned plan(s) should be associated with,
// an optional  ExecutionID and an Extra parameter.
type MoveOnGlobeReq struct {
	ComponentName      resource.Name
	Destination        *geo.Point
	Heading            float64
	MovementSensorName resource.Name
	Obstacles          []*spatialmath.GeoObstacle
	MotionCfg          *MotionConfiguration
	Extra              map[string]interface{}
}

// PlanStep represents a single step of the plan
// Describes the pose each resource described by the plan
// should move to at that step.
type PlanStep map[resource.Name]spatialmath.Pose

// Plan represnts a motion plan.
// Has a unique ID, ComponentName, ExecutionID and a sequence of Steps
// which can be executed to follow the plan.
type Plan struct {
	ID            uuid.UUID
	ComponentName resource.Name
	ExecutionID   uuid.UUID
	Steps         []PlanStep
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

// PlanStatusWithID describes the state of a given plan at a
// point in time plus the PlanId, ComponentName and ExecutionID
// the status is associated with.
type PlanStatusWithID struct {
	PlanID        uuid.UUID
	ComponentName resource.Name
	ExecutionID   uuid.UUID
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
	Plan          Plan
	StatusHistory []PlanStatus
}

// A Service controls the flow of moving components.
type Service interface {
	resource.Resource
	Move(
		ctx context.Context,
		componentName resource.Name,
		destination *referenceframe.PoseInFrame,
		worldState *referenceframe.WorldState,
		constraints *pb.Constraints,
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
	MoveOnGlobeNew(
		ctx context.Context,
		req MoveOnGlobeReq,
	) (string, error)
	GetPose(
		ctx context.Context,
		componentName resource.Name,
		destinationFrame string,
		supplementalTransforms []*referenceframe.LinkInFrame,
		extra map[string]interface{},
	) (*referenceframe.PoseInFrame, error)
	StopPlan(
		ctx context.Context,
		componentName resource.Name,
		extra map[string]interface{},
	) error
	ListPlanStatuses(
		ctx context.Context,
		onlyActivePlans bool,
		extra map[string]interface{},
	) ([]PlanStatusWithID, error)
	PlanHistory(
		ctx context.Context,
		componentName resource.Name,
		lastPlanOnly bool,
		executionID string,
		extra map[string]interface{},
	) ([]PlanWithStatus, error)
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

// ToProto converts a MoveOnGlobeRequest to a *pb.MoveOnGlobeRequest.
//
//nolint:dupl
func (r MoveOnGlobeReq) ToProto(name string) (*pb.MoveOnGlobeRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}

	if r.Destination == nil {
		return nil, errors.New("Must provide a destination")
	}

	if r.MotionCfg == nil {
		return nil, errors.New("Must provide a non nil motion configuration")
	}

	req := &pb.MoveOnGlobeRequest{
		Name:                name,
		ComponentName:       rprotoutils.ResourceNameToProto(r.ComponentName),
		Destination:         &commonpb.GeoPoint{Latitude: r.Destination.Lat(), Longitude: r.Destination.Lng()},
		MovementSensorName:  rprotoutils.ResourceNameToProto(r.MovementSensorName),
		MotionConfiguration: r.MotionCfg.toProto(),
		Extra:               ext,
	}

	if !math.IsNaN(r.Heading) {
		req.Heading = &r.Heading
	}

	if len(r.Obstacles) > 0 {
		obstaclesProto := make([]*commonpb.GeoObstacle, 0, len(r.Obstacles))
		for _, obstacle := range r.Obstacles {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoObstacleToProtobuf(obstacle))
		}
		req.Obstacles = obstaclesProto
	}
	return req, nil
}

// ToProtoNew converts a MoveOnGlobeRequest to a *pb.MoveOnGlobeNewRequest.
//
//nolint:dupl
func (r MoveOnGlobeReq) ToProtoNew(name string) (*pb.MoveOnGlobeNewRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}

	if r.Destination == nil {
		return nil, errors.New("Must provide a destination")
	}

	if r.MotionCfg == nil {
		return nil, errors.New("Must provide a non nil motion configuration")
	}

	req := &pb.MoveOnGlobeNewRequest{
		Name:                name,
		ComponentName:       rprotoutils.ResourceNameToProto(r.ComponentName),
		Destination:         &commonpb.GeoPoint{Latitude: r.Destination.Lat(), Longitude: r.Destination.Lng()},
		MovementSensorName:  rprotoutils.ResourceNameToProto(r.MovementSensorName),
		MotionConfiguration: r.MotionCfg.toProto(),
		Extra:               ext,
	}

	if !math.IsNaN(r.Heading) {
		req.Heading = &r.Heading
	}

	if len(r.Obstacles) > 0 {
		obstaclesProto := make([]*commonpb.GeoObstacle, 0, len(r.Obstacles))
		for _, obstacle := range r.Obstacles {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoObstacleToProtobuf(obstacle))
		}
		req.Obstacles = obstaclesProto
	}
	return req, nil
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
func (p Plan) ToProto() *pb.Plan {
	steps := []*pb.PlanStep{}
	for _, s := range p.Steps {
		steps = append(steps, s.ToProto())
	}

	return &pb.Plan{
		Id:            p.ID.String(),
		ComponentName: rprotoutils.ResourceNameToProto(p.ComponentName),
		ExecutionId:   p.ExecutionID.String(),
		Steps:         steps,
	}
}

// ToProto converts a Step to a *pb.PlanStep.
func (s PlanStep) ToProto() *pb.PlanStep {
	step := make(map[string]*pb.ComponentState)
	for name, pose := range s {
		pbPose := spatialmath.PoseToProtobuf(pose)
		step[name.String()] = &pb.ComponentState{Pose: pbPose}
	}

	return &pb.PlanStep{Step: step}
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
