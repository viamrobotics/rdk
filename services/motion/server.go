package motion

import (
	"context"
	"math"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// serviceServer implements the MotionService from motion.proto.
type serviceServer struct {
	pb.UnimplementedMotionServiceServer
	coll resource.APIResourceCollection[Service]
}

// NewRPCServiceServer constructs a motion gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Service]) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	worldState, err := referenceframe.WorldStateFromProtobuf(req.GetWorldState())
	if err != nil {
		return nil, err
	}
	success, err := svc.Move(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		referenceframe.ProtobufToPoseInFrame(req.GetDestination()),
		worldState,
		req.GetConstraints(),
		req.Extra.AsMap(),
	)
	return &pb.MoveResponse{Success: success}, err
}

func (server *serviceServer) MoveOnMap(ctx context.Context, req *pb.MoveOnMapRequest) (*pb.MoveOnMapResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	success, err := svc.MoveOnMap(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		spatialmath.NewPoseFromProtobuf(req.GetDestination()),
		protoutils.ResourceNameFromProto(req.GetSlamServiceName()),
		req.Extra.AsMap(),
	)
	return &pb.MoveOnMapResponse{Success: success}, err
}

func (server *serviceServer) MoveOnGlobe(ctx context.Context, req *pb.MoveOnGlobeRequest) (*pb.MoveOnGlobeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if req.Destination == nil {
		return nil, errors.New("Must provide a destination")
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
			return nil, err
		}
		obstacles = append(obstacles, convObst)
	}

	success, err := svc.MoveOnGlobe(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		geo.NewPoint(req.GetDestination().GetLatitude(), req.GetDestination().GetLongitude()),
		heading,
		protoutils.ResourceNameFromProto(req.GetMovementSensorName()),
		obstacles,
		motionConfigurationFromProto(req.MotionConfiguration),
		req.Extra.AsMap(),
	)
	return &pb.MoveOnGlobeResponse{Success: success}, err
}

func (server *serviceServer) MoveOnGlobeNew(ctx context.Context, req *pb.MoveOnGlobeRequest) (*pb.MoveOnGlobeNewResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if req.Destination == nil {
		return nil, errors.New("Must provide a destination")
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
			return nil, err
		}
		obstacles = append(obstacles, convObst)
	}

	id, err := svc.MoveOnGlobeNew(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		geo.NewPoint(req.GetDestination().GetLatitude(), req.GetDestination().GetLongitude()),
		heading,
		protoutils.ResourceNameFromProto(req.GetMovementSensorName()),
		obstacles,
		motionConfigurationFromProto(req.MotionConfiguration),
		req.Extra.AsMap(),
	)
	return &pb.MoveOnGlobeNewResponse{ExecutionId: id}, err
}

func (server *serviceServer) GetPose(ctx context.Context, req *pb.GetPoseRequest) (*pb.GetPoseResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if req.ComponentName == nil {
		return nil, errors.New("must provide component name")
	}
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	pose, err := svc.GetPose(ctx, protoutils.ResourceNameFromProto(req.ComponentName), req.DestinationFrame, transforms, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPoseResponse{Pose: referenceframe.PoseInFrameToProtobuf(pose)}, nil
}

func (server *serviceServer) StopPlan(ctx context.Context, req *pb.StopPlanRequest) (*pb.StopPlanResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopPlanResponse{}, svc.StopPlan(ctx, protoutils.ResourceNameFromProto(req.GetRootComponent()), req.Extra.AsMap())
}

func (server *serviceServer) ListPlanStatuses(ctx context.Context, req *pb.ListPlanStatusesRequest) (*pb.ListPlanStatusesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	statuses, err := svc.ListPlanStatuses(ctx, req.GetOnlyActivePlans(), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	protoStatuses := make([]*pb.PlanStatus, 0, len(statuses))
	for _, status := range statuses {
		protoStatuses = append(protoStatuses, planStatusToProto(status))
	}
	return &pb.ListPlanStatusesResponse{Statuses: protoStatuses}, nil
}

func (server *serviceServer) GetPlan(ctx context.Context, req *pb.GetPlanRequest) (*pb.GetPlanResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	planHistory, err := svc.CurrentPlanHistory(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		req.GetLastPlanOnly(),
		req.GetExecutionId(),
		req.Extra.AsMap(),
	)
	if err != nil {
		return nil, err
	}
	history := []*pb.PlanWithStatus{}
	if len(planHistory) > 1 {
		for _, plan := range planHistory[1:] {
			history = append(history, planWithStatusToProto(plan))
		}
	}
	return &pb.GetPlanResponse{CurrentPlanWithStatus: planWithStatusToProto(planHistory[0]), ReplanHistory: history}, nil
}

// DoCommand receives arbitrary commands.
func (server *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, svc, req)
}
