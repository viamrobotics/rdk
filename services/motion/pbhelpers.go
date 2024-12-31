package motion

import (
	"math"

	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"

	"go.viam.com/rdk/motionplan"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// ToProto converts a MoveReq to a pb.MoveRequest
// the name argument should correspond to the name of the motion service the request will be used with.
func (r MoveReq) ToProto(name string) (*pb.MoveRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}
	worldStateMsg, err := r.WorldState.ToProtobuf()
	if err != nil {
		return nil, err
	}
	reqPB := &pb.MoveRequest{
		Name:          name,
		ComponentName: rprotoutils.ResourceNameToProto(r.ComponentName),
		WorldState:    worldStateMsg,
		Constraints:   r.Constraints.ToProtobuf(),
		Extra:         ext,
	}
	if r.Destination != nil {
		// Destination is not needed if goal_state present. Validation on receiving end.
		reqPB.Destination = referenceframe.PoseInFrameToProtobuf(r.Destination)
	}

	return reqPB, nil
}

// MoveReqFromProto converts a pb.MoveRequest to a MoveReq struct.
func MoveReqFromProto(req *pb.MoveRequest) (MoveReq, error) {
	worldState, err := referenceframe.WorldStateFromProtobuf(req.GetWorldState())
	if err != nil {
		return MoveReq{}, err
	}
	dst := req.GetDestination()
	var destination *referenceframe.PoseInFrame
	if dst != nil {
		// Destination is not needed if goal_state present. Validation on receiving end.
		destination = referenceframe.ProtobufToPoseInFrame(req.GetDestination())
	}

	return MoveReq{
		rprotoutils.ResourceNameFromProto(req.GetComponentName()),
		destination,
		worldState,
		motionplan.ConstraintsFromProtobuf(req.GetConstraints()),
		req.Extra.AsMap(),
	}, nil
}

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
func planFromProto(p *pb.Plan) (PlanWithMetadata, error) {
	if p == nil {
		return PlanWithMetadata{}, errors.New("received nil *pb.Plan")
	}

	id, err := uuid.Parse(p.Id)
	if err != nil {
		return PlanWithMetadata{}, err
	}

	executionID, err := uuid.Parse(p.ExecutionId)
	if err != nil {
		return PlanWithMetadata{}, err
	}

	if p.ComponentName == nil {
		return PlanWithMetadata{}, errors.New("received nil *pb.ResourceName")
	}

	plan := PlanWithMetadata{
		ID:            id,
		ComponentName: rprotoutils.ResourceNameFromProto(p.ComponentName),
		ExecutionID:   executionID,
	}

	if len(p.Steps) == 0 {
		return plan, nil
	}

	steps := motionplan.Path{}
	for _, s := range p.Steps {
		step, err := motionplan.FrameSystemPosesFromProto(s)
		if err != nil {
			return PlanWithMetadata{}, err
		}
		steps = append(steps, step)
	}
	plan.Plan = motionplan.NewSimplePlan(steps, nil)
	return plan, nil
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
		obstaclesProto := make([]*commonpb.GeoGeometry, 0, len(r.Obstacles))
		for _, obstacle := range r.Obstacles {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoGeometryToProtobuf(obstacle))
		}
		req.Obstacles = obstaclesProto
	}
	if len(r.BoundingRegions) > 0 {
		obstaclesProto := make([]*commonpb.GeoGeometry, 0, len(r.BoundingRegions))
		for _, obstacle := range r.BoundingRegions {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoGeometryToProtobuf(obstacle))
		}
		req.BoundingRegions = obstaclesProto
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
	obstacles := make([]*spatialmath.GeoGeometry, 0, len(obstaclesProto))
	for _, eachProtoObst := range obstaclesProto {
		convObst, err := spatialmath.GeoGeometryFromProtobuf(eachProtoObst)
		if err != nil {
			return MoveOnGlobeReq{}, err
		}
		obstacles = append(obstacles, convObst)
	}

	boundingRegionGeometriesProto := req.GetBoundingRegions()
	boundingRegionGeometries := make([]*spatialmath.GeoGeometry, 0, len(boundingRegionGeometriesProto))
	for _, eachProtoObst := range boundingRegionGeometriesProto {
		convObst, err := spatialmath.GeoGeometryFromProtobuf(eachProtoObst)
		if err != nil {
			return MoveOnGlobeReq{}, err
		}
		boundingRegionGeometries = append(boundingRegionGeometries, convObst)
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
		BoundingRegions:    boundingRegionGeometries,
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

func moveOnMapRequestFromProto(req *pb.MoveOnMapRequest) (MoveOnMapReq, error) {
	if req == nil {
		return MoveOnMapReq{}, errors.New("received nil *pb.MoveOnMapRequest")
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
	geoms := []spatialmath.Geometry{}
	if obs := req.GetObstacles(); len(obs) > 0 {
		convertedGeom, err := spatialmath.NewGeometriesFromProto(obs)
		if err != nil {
			return MoveOnMapReq{}, errors.Wrap(err, "cannot convert obstacles into geometries")
		}
		geoms = convertedGeom
	}
	return MoveOnMapReq{
		ComponentName: rprotoutils.ResourceNameFromProto(protoComponentName),
		Destination:   spatialmath.NewPoseFromProtobuf(req.GetDestination()),
		SlamName:      rprotoutils.ResourceNameFromProto(protoSlamServiceName),
		MotionCfg:     configurationFromProto(req.MotionConfiguration),
		Obstacles:     geoms,
		Extra:         req.Extra.AsMap(),
	}, nil
}

func (r MoveOnMapReq) toProto(name string) (*pb.MoveOnMapRequest, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return nil, err
	}
	if r.Destination == nil {
		return nil, errors.New("must provide a destination")
	}
	req := &pb.MoveOnMapRequest{
		Name:            name,
		ComponentName:   rprotoutils.ResourceNameToProto(r.ComponentName),
		Destination:     spatialmath.PoseToProtobuf(r.Destination),
		SlamServiceName: rprotoutils.ResourceNameToProto(r.SlamName),
		Obstacles:       spatialmath.NewGeometriesToProto(r.Obstacles),
		Extra:           ext,
	}

	if r.MotionCfg != nil {
		req.MotionConfiguration = r.MotionCfg.toProto()
	}

	return req, nil
}
