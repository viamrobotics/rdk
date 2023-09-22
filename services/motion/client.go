package motion

import (
	"context"
	"math"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
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

func (c *client) MoveOnGlobeNew(
	ctx context.Context,
	componentName resource.Name,
	destination *geo.Point,
	heading float64,
	movementSensorName resource.Name,
	obstacles []*spatialmath.GeoObstacle,
	motionCfg *MotionConfiguration,
	extra map[string]interface{},

	// note from Ray it feels weird that we're returning a string here when its an "id"

) (string, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return "", err
	}

	if destination == nil {
		return "", errors.New("Must provide a destination")
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
		return "", err
	}

	return resp.ExecutionId, nil
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

func (c *client) StopPlan(ctx context.Context, rootComponent resource.Name, extra map[string]interface{}) error {
	return grpc.UnimplementedError
}

func (c *client) ListPlanStatuses(ctx context.Context, onlyActivePlans bool, extra map[string]interface{}) ([]PlanStatus, error) {
	return nil, grpc.UnimplementedError
}

func (c *client) Plan(
	ctx context.Context,
	componentName resource.Name,
	lastPlanOnly bool,
	executionID string,
	extra map[string]interface{},
) ([]PlanWithStatus, error) {
	return nil, grpc.UnimplementedError
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
