package motion

import (
	"errors"
	"math"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"

	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// planWithStatusFromProto converts a *pb.PlanWithStatus to a PlanWithStatus.
func planWithStatusFromProto(pws *pb.PlanWithStatus) (PlanWithStatus, error) {
	if pws == nil {
		return PlanWithStatus{}, errors.New("received nil *pb.PlanWithStatus")
	}

	plan, err := planFromProto(pws.Plan)
	if err != nil {
		return PlanWithStatus{}, err
	}

	status, err := planStatusFromProto(pws.Status)
	if err != nil {
		return PlanWithStatus{}, err
	}
	statusHistory := []PlanStatus{}
	statusHistory = append(statusHistory, status)
	for _, s := range pws.StatusHistory {
		ps, err := planStatusFromProto(s)
		if err != nil {
			return PlanWithStatus{}, err
		}
		statusHistory = append(statusHistory, ps)
	}

	return PlanWithStatus{
		Plan:          plan,
		StatusHistory: statusHistory,
	}, nil
}

// planStatusFromProto converts a *pb.PlanStatus to a PlanStatus.
func planStatusFromProto(ps *pb.PlanStatus) (PlanStatus, error) {
	if ps == nil {
		return PlanStatus{}, errors.New("received nil *pb.PlanStatus")
	}

	return PlanStatus{
		State:     planStateFromProto(ps.State),
		Reason:    ps.Reason,
		Timestamp: ps.Timestamp.AsTime(),
	}, nil
}

// planStatusWithIDFromProto converts a *pb.PlanStatus to a PlanStatus.
func planStatusWithIDFromProto(ps *pb.PlanStatusWithID) (PlanStatusWithID, error) {
	if ps == nil {
		return PlanStatusWithID{}, errors.New("received nil *pb.PlanStatusWithID")
	}

	planID, err := uuid.Parse(ps.PlanId)
	if err != nil {
		return PlanStatusWithID{}, err
	}

	executionID, err := uuid.Parse(ps.ExecutionId)
	if err != nil {
		return PlanStatusWithID{}, err
	}

	status, err := planStatusFromProto(ps.Status)
	if err != nil {
		return PlanStatusWithID{}, err
	}

	if ps.ComponentName == nil {
		return PlanStatusWithID{}, errors.New("received nil *commonpb.ResourceName")
	}

	return PlanStatusWithID{
		PlanID:        planID,
		ComponentName: rprotoutils.ResourceNameFromProto(ps.ComponentName),
		ExecutionID:   executionID,
		Status:        status,
	}, nil
}

// planFromProto converts a *pb.Plan to a Plan.
func planFromProto(p *pb.Plan) (Plan, error) {
	if p == nil {
		return Plan{}, errors.New("received nil *pb.Plan")
	}

	id, err := uuid.Parse(p.Id)
	if err != nil {
		return Plan{}, err
	}

	executionID, err := uuid.Parse(p.ExecutionId)
	if err != nil {
		return Plan{}, err
	}

	if p.ComponentName == nil {
		return Plan{}, errors.New("received nil *pb.ResourceName")
	}

	plan := Plan{
		ID:            id,
		ComponentName: rprotoutils.ResourceNameFromProto(p.ComponentName),
		ExecutionID:   executionID,
	}

	if len(p.Steps) == 0 {
		return plan, nil
	}

	steps := []PlanStep{}
	for _, s := range p.Steps {
		step, err := planStepFromProto(s)
		if err != nil {
			return Plan{}, err
		}
		steps = append(steps, step)
	}

	plan.Steps = steps

	return plan, nil
}

// planStepFromProto converts a *pb.PlanStep to a PlanStep.
func planStepFromProto(s *pb.PlanStep) (PlanStep, error) {
	if s == nil {
		return PlanStep{}, errors.New("received nil *pb.PlanStep")
	}

	step := make(PlanStep)
	for k, v := range s.Step {
		name, err := resource.NewFromString(k)
		if err != nil {
			return PlanStep{}, err
		}
		step[name] = spatialmath.NewPoseFromProtobuf(v.Pose)
	}
	return step, nil
}

// planStateFromProto converts a pb.PlanState to a PlanState.
func planStateFromProto(ps pb.PlanState) PlanState {
	switch ps {
	case pb.PlanState_PLAN_STATE_IN_PROGRESS:
		return PlanStateInProgress
	case pb.PlanState_PLAN_STATE_STOPPED:
		return PlanStateStopped
	case pb.PlanState_PLAN_STATE_SUCCEEDED:
		return PlanStateSucceeded
	case pb.PlanState_PLAN_STATE_FAILED:
		return PlanStateFailed
	case pb.PlanState_PLAN_STATE_UNSPECIFIED:
		return PlanStateUnspecified
	default:
		return PlanStateUnspecified
	}
}

// toProto converts a MoveOnGlobeRequest to a *pb.MoveOnGlobeRequest.
func (r MoveOnGlobeReq) toProto(name string) (*pb.MoveOnGlobeRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}

	if r.Destination == nil {
		return nil, errors.New("must provide a destination")
	}

	req := &pb.MoveOnGlobeRequest{
		Name:               name,
		ComponentName:      rprotoutils.ResourceNameToProto(r.ComponentName),
		Destination:        &commonpb.GeoPoint{Latitude: r.Destination.Lat(), Longitude: r.Destination.Lng()},
		MovementSensorName: rprotoutils.ResourceNameToProto(r.MovementSensorName),
		Extra:              ext,
	}

	if !math.IsNaN(r.Heading) {
		req.Heading = &r.Heading
	}

	if r.MotionCfg != nil {
		req.MotionConfiguration = r.MotionCfg.toProto()
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

func moveOnGlobeRequestFromProto(req *pb.MoveOnGlobeRequest) (MoveOnGlobeReq, error) {
	if req == nil {
		return MoveOnGlobeReq{}, errors.New("received nil *pb.MoveOnGlobeRequest")
	}

	if req.Destination == nil {
		return MoveOnGlobeReq{}, errors.New("must provide a destination")
	}

	// Optionals
	heading := math.NaN()
	if req.Heading != nil {
		heading = req.GetHeading()
	}
	obstaclesProto := req.GetObstacles()
	obstacles := make([]*spatialmath.GeoObstacle, 0, len(obstaclesProto))
	for _, eachProtoObst := range obstaclesProto {
		convObst, err := spatialmath.GeoObstacleFromProtobuf(eachProtoObst)
		if err != nil {
			return MoveOnGlobeReq{}, err
		}
		obstacles = append(obstacles, convObst)
	}
	protoComponentName := req.GetComponentName()
	if protoComponentName == nil {
		return MoveOnGlobeReq{}, errors.New("received nil *commonpb.ResourceName")
	}
	componentName := rprotoutils.ResourceNameFromProto(protoComponentName)
	destination := geo.NewPoint(req.GetDestination().GetLatitude(), req.GetDestination().GetLongitude())
	protoMovementSensorName := req.GetMovementSensorName()
	if protoMovementSensorName == nil {
		return MoveOnGlobeReq{}, errors.New("received nil *commonpb.ResourceName")
	}
	movementSensorName := rprotoutils.ResourceNameFromProto(protoMovementSensorName)
	motionCfg := configurationFromProto(req.MotionConfiguration)

	return MoveOnGlobeReq{
		ComponentName:      componentName,
		Destination:        destination,
		Heading:            heading,
		MovementSensorName: movementSensorName,
		Obstacles:          obstacles,
		MotionCfg:          motionCfg,
		Extra:              req.Extra.AsMap(),
	}, nil
}

func (req PlanHistoryReq) toProto(name string) (*pb.GetPlanRequest, error) {
	ext, err := vprotoutils.StructToStructPb(req.Extra)
	if err != nil {
		return nil, err
	}

	var executionIDPtr *string
	if req.ExecutionID != uuid.Nil {
		executionID := req.ExecutionID.String()
		executionIDPtr = &executionID
	}
	return &pb.GetPlanRequest{
		Name:          name,
		ComponentName: rprotoutils.ResourceNameToProto(req.ComponentName),
		LastPlanOnly:  req.LastPlanOnly,
		Extra:         ext,
		ExecutionId:   executionIDPtr,
	}, nil
}

func getPlanRequestFromProto(req *pb.GetPlanRequest) (PlanHistoryReq, error) {
	if req.GetComponentName() == nil {
		return PlanHistoryReq{}, errors.New("received nil *commonpb.ResourceName")
	}

	executionID := uuid.Nil
	if executionIDStr := req.GetExecutionId(); executionIDStr != "" {
		id, err := uuid.Parse(executionIDStr)
		if err != nil {
			return PlanHistoryReq{}, err
		}
		executionID = id
	}

	return PlanHistoryReq{
		ComponentName: rprotoutils.ResourceNameFromProto(req.GetComponentName()),
		LastPlanOnly:  req.GetLastPlanOnly(),
		ExecutionID:   executionID,
		Extra:         req.Extra.AsMap(),
	}, nil
}

func moveOnMapNewRequestFromProto(req *pb.MoveOnMapNewRequest) (MoveOnMapReq, error) {
	if req == nil {
		return MoveOnMapReq{}, errors.New("received nil *pb.MoveOnMapNewRequest")
	}
	if req.GetDestination() == nil {
		return MoveOnMapReq{}, errors.New("received nil *commonpb.Pose for destination")
	}
	protoComponentName := req.GetComponentName()
	if protoComponentName == nil {
		return MoveOnMapReq{}, errors.New("received nil *commonpb.ResourceName for component name")
	}
	protoSlamServiceName := req.GetSlamServiceName()
	if protoSlamServiceName == nil {
		return MoveOnMapReq{}, errors.New("received nil *commonpb.ResourceName for SlamService name")
	}
	return MoveOnMapReq{
		ComponentName: rprotoutils.ResourceNameFromProto(protoComponentName),
		Destination:   spatialmath.NewPoseFromProtobuf(req.GetDestination()),
		SlamName:      rprotoutils.ResourceNameFromProto(protoSlamServiceName),
		MotionCfg:     configurationFromProto(req.MotionConfiguration),
		Extra:         req.Extra.AsMap(),
	}, nil
}

func (r MoveOnMapReq) toProtoNew(name string) (*pb.MoveOnMapNewRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}
	if r.Destination == nil {
		return nil, errors.New("must provide a destination")
	}

	req := &pb.MoveOnMapNewRequest{
		Name:            name,
		ComponentName:   rprotoutils.ResourceNameToProto(r.ComponentName),
		Destination:     spatialmath.PoseToProtobuf(r.Destination),
		SlamServiceName: rprotoutils.ResourceNameToProto(r.SlamName),
		Extra:           ext,
	}

	if r.MotionCfg != nil {
		req.MotionConfiguration = r.MotionCfg.toProto()
	}

	return req, nil
}
