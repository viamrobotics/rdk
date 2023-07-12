// Package motion contains a gRPC based motion client
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
	motionCfg MotionConfiguration,
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
		for _, eachObst := range obstacles {
			convObst, err := spatialmath.GeoObstacleToProtobuf(eachObst)
			if err != nil {
				return false, err
			}
			obstaclesProto = append(obstaclesProto, convObst)
		}
		req.Obstacles = obstaclesProto
	}

	if !math.IsNaN(motionCfg.AngularMetersPerSec) {
		req.MotionConfiguration.AngularDegPerSec = &motionCfg.AngularMetersPerSec
	}
	if !math.IsNaN(motionCfg.LinearMetersPerSec) {
		req.MotionConfiguration.LinearMetersPerSec = &motionCfg.LinearMetersPerSec
	}
	if !math.IsNaN(motionCfg.ObstaclePollingFreq) {
		req.MotionConfiguration.ObstaclePollingFrequency = &motionCfg.ObstaclePollingFreq
	}
	if !math.IsNaN(motionCfg.PlanDeviationMeters) {
		req.MotionConfiguration.PlanDeviationMeters = &motionCfg.PlanDeviationMeters
	}
	if !math.IsNaN(motionCfg.PositionPollingFreq) {
		req.MotionConfiguration.PositionPollingFrequency = &motionCfg.PositionPollingFreq
	}
	if !math.IsNaN(motionCfg.ReplanCostFactor) {
		req.MotionConfiguration.ReplanCostFactor = &motionCfg.ReplanCostFactor
	}
	if len(motionCfg.VisionSvc) > 0 {
		svcs := []*commonpb.ResourceName{}
		for _, name := range motionCfg.VisionSvc {
			svcs = append(svcs, protoutils.ResourceNameToProto(name))
		}
		req.MotionConfiguration.VisionServices = svcs
	}
	if motionCfg.Extra != nil {
		structPB, err := vprotoutils.StructToStructPb(motionCfg.Extra)
		if err != nil {
			return false, err
		}
		req.MotionConfiguration.Extra = structPB

	}

	resp, err := c.client.MoveOnGlobe(ctx, req)
	if err != nil {
		return false, err
	}

	return resp.Success, nil
}

func (c *client) MoveSingleComponent(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
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
	resp, err := c.client.MoveSingleComponent(ctx, &pb.MoveSingleComponentRequest{
		Name:          c.name,
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldStateMsg,
		Extra:         ext,
	})
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

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
