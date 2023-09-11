package motion

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// client implements MotionServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.MotionServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger golog.Logger,
) (Service, error) {
	grpcClient := pb.NewMotionServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *pb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	worldStateMsg, err := worldState.ToProtobuf()
	if err != nil {
		return false, err
	}
	resp, err := c.client.Move(ctx, &pb.MoveRequest{
		Name:          c.name,
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldStateMsg,
		Constraints:   constraints,
		Extra:         ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) MoveOnMap(
	ctx context.Context,
	componentName resource.Name,
	destination spatialmath.Pose,
	slamName resource.Name,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	resp, err := c.client.MoveOnMap(ctx, &pb.MoveOnMapRequest{
		Name:            c.name,
		ComponentName:   protoutils.ResourceNameToProto(componentName),
		Destination:     spatialmath.PoseToProtobuf(destination),
		SlamServiceName: protoutils.ResourceNameToProto(slamName),
		Extra:           ext,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) MoveOnGlobe(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *MotionConfiguration,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}

	if destination == nil {
		return false, errors.New("Must provide a destination")
	}

	req := &pb.MoveOnGlobeRequest{
		Name:                c.name,
		ComponentName:       protoutils.ResourceNameToProto(componentName),
		Destination:         &commonpb.GeoPoint{Latitude: destination.Lat(), Longitude: destination.Lng()},
		MovementSensorName:  protoutils.ResourceNameToProto(movementSensorName),
		MotionConfiguration: &pb.MotionConfiguration{},
		Extra:               ext,
	}

	// Optionals
	if !math.IsNaN(heading) {
		req.Heading = &heading
	}
	if len(obstacles) > 0 {
		obstaclesProto := make([]*commonpb.GeoObstacle, 0, len(obstacles))
		for _, obstacle := range obstacles {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoObstacleToProtobuf(obstacle))
		}
		req.Obstacles = obstaclesProto
	}

	if !math.IsNaN(motionCfg.LinearMPerSec) && motionCfg.LinearMPerSec != 0 {
		req.MotionConfiguration.LinearMPerSec = &motionCfg.LinearMPerSec
	}
	if !math.IsNaN(motionCfg.AngularDegsPerSec) && motionCfg.AngularDegsPerSec != 0 {
		req.MotionConfiguration.AngularDegsPerSec = &motionCfg.AngularDegsPerSec
	}
	if !math.IsNaN(motionCfg.ObstaclePollingFreqHz) && motionCfg.ObstaclePollingFreqHz > 0 {
		req.MotionConfiguration.ObstaclePollingFrequencyHz = &motionCfg.ObstaclePollingFreqHz
	}
	if !math.IsNaN(motionCfg.PositionPollingFreqHz) && motionCfg.PositionPollingFreqHz > 0 {
		req.MotionConfiguration.PositionPollingFrequencyHz = &motionCfg.PositionPollingFreqHz
	}
	if !math.IsNaN(motionCfg.PlanDeviationMM) && motionCfg.PlanDeviationMM >= 0 {
		planDeviationM := 1e-3 * motionCfg.PlanDeviationMM
		req.MotionConfiguration.PlanDeviationM = &planDeviationM
	}

	if len(motionCfg.VisionServices) > 0 {
		svcs := []*commonpb.ResourceName{}
		for _, name := range motionCfg.VisionServices {
			svcs = append(svcs, protoutils.ResourceNameToProto(name))
		}
		req.MotionConfiguration.VisionServices = svcs
	}

	resp, err := c.client.MoveOnGlobe(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

func (c *client) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
	supplementalTransforms []*referenceframe.LinkInFrame,
	extra map[string]interface{},
) (*referenceframe.PoseInFrame, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	transforms, err := referenceframe.LinkInFramesToTransformsProtobuf(supplementalTransforms)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetPose(ctx, &pb.GetPoseRequest{
		Name:                   c.name,
		ComponentName:          protoutils.ResourceNameToProto(componentName),
		DestinationFrame:       destinationFrame,
		SupplementalTransforms: transforms,
		Extra:                  ext,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

func (c *client) MoveOnGlobeNew(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *MotionConfiguration,
	extra map[string]interface{},
) (uuid.UUID, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return uuid.Nil, err
	}

	if destination == nil {
		return uuid.Nil, errors.New("Must provide a destination")
	}

	req := &pb.MoveOnGlobeNewRequest{
		Name:                c.name,
		ComponentName:       protoutils.ResourceNameToProto(componentName),
		Destination:         &commonpb.GeoPoint{Latitude: destination.Lat(), Longitude: destination.Lng()},
		MovementSensorName:  protoutils.ResourceNameToProto(movementSensorName),
		MotionConfiguration: &pb.MotionConfiguration{},
		Extra:               ext,
	}

	// Optionals
	if !math.IsNaN(heading) {
		req.Heading = &heading
	}
	if len(obstacles) > 0 {
		obstaclesProto := make([]*commonpb.GeoObstacle, 0, len(obstacles))
		for _, obstacle := range obstacles {
			obstaclesProto = append(obstaclesProto, spatialmath.GeoObstacleToProtobuf(obstacle))
		}
		req.Obstacles = obstaclesProto
	}

	if !math.IsNaN(motionCfg.LinearMPerSec) && motionCfg.LinearMPerSec != 0 {
		req.MotionConfiguration.LinearMPerSec = &motionCfg.LinearMPerSec
	}
	if !math.IsNaN(motionCfg.AngularDegsPerSec) && motionCfg.AngularDegsPerSec != 0 {
		req.MotionConfiguration.AngularDegsPerSec = &motionCfg.AngularDegsPerSec
	}
	if !math.IsNaN(motionCfg.ObstaclePollingFreqHz) && motionCfg.ObstaclePollingFreqHz > 0 {
		req.MotionConfiguration.ObstaclePollingFrequencyHz = &motionCfg.ObstaclePollingFreqHz
	}
	if !math.IsNaN(motionCfg.PositionPollingFreqHz) && motionCfg.PositionPollingFreqHz > 0 {
		req.MotionConfiguration.PositionPollingFrequencyHz = &motionCfg.PositionPollingFreqHz
	}
	if !math.IsNaN(motionCfg.PlanDeviationMM) && motionCfg.PlanDeviationMM >= 0 {
		planDeviationM := 1e-3 * motionCfg.PlanDeviationMM
		req.MotionConfiguration.PlanDeviationM = &planDeviationM
	}

	if len(motionCfg.VisionServices) > 0 {
		svcs := []*commonpb.ResourceName{}
		for _, name := range motionCfg.VisionServices {
			svcs = append(svcs, protoutils.ResourceNameToProto(name))
		}
		req.MotionConfiguration.VisionServices = svcs
	}

	resp, err := c.client.MoveOnGlobeNew(ctx, req)
	if err != nil {
		return uuid.Nil, err
	}
	opid, err := uuid.Parse(resp.OperationId)
	if err != nil {
		return uuid.Nil, err
	}

	return opid, nil
}

func (c *client) ListPlanStatuses(
	ctx context.Context,
	extra map[string]interface{},
) ([]PlanStatus, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	req := &pb.ListPlanStatusesRequest{
		Name:  c.name,
		Extra: ext,
	}

	resp, err := c.client.ListPlanStatuses(ctx, req)
	if err != nil {
		return nil, err
	}

	statuses := []PlanStatus{}
	for _, s := range resp.Statuses {
		ps, err := toPlanStatus(s)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, ps)
	}
	return statuses, nil
}

func (c *client) GetPlan(
	ctx context.Context,
	r GetPlanRequest,
) (OpIDPlans, error) {
	ext, err := vprotoutils.StructToStructPb(r.Extra)
	if err != nil {
		return OpIDPlans{}, err
	}

	req := &pb.GetPlanRequest{
		Name:        c.name,
		OperationId: r.OperationID.String(),
		Extra:       ext,
	}

	resp, err := c.client.GetPlan(ctx, req)
	if err != nil {
		return OpIDPlans{}, err
	}

	current, err := toPlanWithStatus(resp.CurrentPlanWithStatus)
	if err != nil {
		return OpIDPlans{}, err
	}

	replanHistory := []PlanWithStatus{}
	for _, pws := range resp.ReplanHistory {
		p, err := toPlanWithStatus(pws)
		if err != nil {
			return OpIDPlans{}, err
		}
		replanHistory = append(replanHistory, p)
	}

	return OpIDPlans{
		CurrentPlanWithPlanWithStatus: current,
		ReplanHistory:                 replanHistory,
	}, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func toPlan(p *pb.Plan) (Plan, error) {
	id, err := uuid.Parse(p.Id)
	if err != nil {
		return Plan{}, err
	}

	steps := []Step{}
	for _, s := range p.Steps {
		step := make(Step)
		for k, v := range s.Step {
			name, err := resource.NewFromString(k)
			if err != nil {
				return Plan{}, err
			}
			step[name] = spatialmath.NewPoseFromProtobuf(v.Pose)
		}
		steps = append(steps, step)
	}
	return Plan{ID: id, Steps: steps}, nil
}

func toPlanStatus(ps *pb.PlanStatus) (PlanStatus, error) {
	planID, err := uuid.Parse(ps.PlanId)
	if err != nil {
		return PlanStatus{}, err
	}

	opid, err := uuid.Parse(ps.OperationId)
	if err != nil {
		return PlanStatus{}, err
	}

	var reason string
	if ps.Reason != nil {
		reason = *ps.Reason
	}

	return PlanStatus{
		PlanID:      planID,
		OperationID: opid,
		State:       int32(ps.State.Number()),
		Reason:      reason,
		Timestamp:   ps.Timestamp.AsTime(),
	}, nil
}

func toPlanWithStatus(pws *pb.PlanWithStatus) (PlanWithStatus, error) {
	plan, err := toPlan(pws.Plan)
	if err != nil {
		return PlanWithStatus{}, err
	}

	status, err := toPlanStatus(pws.Status)
	if err != nil {
		return PlanWithStatus{}, err
	}

	statusHistory := []PlanStatus{}
	for _, s := range pws.StatusHistory {
		ps, err := toPlanStatus(s)
		if err != nil {
			return PlanWithStatus{}, err
		}
		statusHistory = append(statusHistory, ps)
	}

	return PlanWithStatus{
		Plan:          plan,
		Status:        status,
		StatusHistory: statusHistory,
	}, nil
}
