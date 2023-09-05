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
	motionCfg := setupMotionConfiguration(req.MotionConfiguration)

	success, err := svc.MoveOnGlobe(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		geo.NewPoint(req.GetDestination().GetLatitude(), req.GetDestination().GetLongitude()),
		heading,
		protoutils.ResourceNameFromProto(req.GetMovementSensorName()),
		obstacles,
		&motionCfg,
		req.Extra.AsMap(),
	)
	return &pb.MoveOnGlobeResponse{Success: success}, err
}

func setupMotionConfiguration(motionCfg *pb.MotionConfiguration) MotionConfiguration {
	visionSvc := []resource.Name{}
	planDeviationM := 0.
	positionPollingHz := 0.
	obstaclePollingHz := 0.
	linearMPerSec := 0.
	angularDegsPerSec := 0.

	if motionCfg != nil {
		if motionCfg.VisionServices != nil {
			for _, name := range motionCfg.GetVisionServices() {
				visionSvc = append(visionSvc, protoutils.ResourceNameFromProto(name))
			}
		}
		if motionCfg.PositionPollingFrequencyHz != nil {
			positionPollingHz = motionCfg.GetPositionPollingFrequencyHz()
		}
		if motionCfg.ObstaclePollingFrequencyHz != nil {
			obstaclePollingHz = motionCfg.GetObstaclePollingFrequencyHz()
		}
		if motionCfg.PlanDeviationM != nil {
			planDeviationM = motionCfg.GetPlanDeviationM()
		}
		if motionCfg.LinearMPerSec != nil {
			linearMPerSec = motionCfg.GetLinearMPerSec()
		}
		if motionCfg.AngularDegsPerSec != nil {
			angularDegsPerSec = motionCfg.GetAngularDegsPerSec()
		}
	}

	return MotionConfiguration{
		VisionServices:        visionSvc,
		PositionPollingFreqHz: positionPollingHz,
		ObstaclePollingFreqHz: obstaclePollingHz,
		PlanDeviationMM:       1e3 * planDeviationM,
		LinearMPerSec:         linearMPerSec,
		AngularDegsPerSec:     angularDegsPerSec,
	}
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
