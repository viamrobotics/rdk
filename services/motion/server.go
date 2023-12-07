package motion

import (
	"context"

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

func (server *serviceServer) MoveOnMapNew(ctx context.Context, req *pb.MoveOnMapNewRequest) (*pb.MoveOnMapNewResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	id, err := svc.MoveOnMapNew(
		ctx,
		protoutils.ResourceNameFromProto(req.GetComponentName()),
		spatialmath.NewPoseFromProtobuf(req.GetDestination()),
		protoutils.ResourceNameFromProto(req.GetSlamServiceName()),
		configurationFromProto(req.GetMotionConfiguration()),
		req.Extra.AsMap(),
	)
	return &pb.MoveOnMapNewResponse{ExecutionId: id.String()}, err
}

// NOTE: Ignoring duplication as we are going to delete the current (blocking) implementation of MoveOnGlobe after the
// "Expose Paths To Users" project is complete
//

func (server *serviceServer) MoveOnGlobe(ctx context.Context, req *pb.MoveOnGlobeRequest) (*pb.MoveOnGlobeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	r, err := moveOnGlobeRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	success, err := svc.MoveOnGlobe(
		ctx,
		r.ComponentName,
		r.Destination,
		r.Heading,
		r.MovementSensorName,
		r.Obstacles,
		r.MotionCfg,
		r.Extra,
	)
	return &pb.MoveOnGlobeResponse{Success: success}, err
}

// NOTE: Ignoring duplication as we are going to delete the current (blocking) implementation of MoveOnGlobe after the
// "Expose Paths To Users" project is complete
//

func (server *serviceServer) MoveOnGlobeNew(ctx context.Context, req *pb.MoveOnGlobeNewRequest) (*pb.MoveOnGlobeNewResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	r, err := moveOnGlobeNewRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	id, err := svc.MoveOnGlobeNew(ctx, r)
	if err != nil {
		return nil, err
	}

	return &pb.MoveOnGlobeNewResponse{ExecutionId: id.String()}, nil
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

	componentName := protoutils.ResourceNameFromProto(req.GetComponentName())
	r := StopPlanReq{ComponentName: componentName, Extra: req.Extra.AsMap()}
	err = svc.StopPlan(ctx, r)
	if err != nil {
		return nil, err
	}

	return &pb.StopPlanResponse{}, nil
}

func (server *serviceServer) ListPlanStatuses(ctx context.Context, req *pb.ListPlanStatusesRequest) (*pb.ListPlanStatusesResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	r := ListPlanStatusesReq{OnlyActivePlans: req.GetOnlyActivePlans(), Extra: req.Extra.AsMap()}
	statuses, err := svc.ListPlanStatuses(ctx, r)
	if err != nil {
		return nil, err
	}

	protoStatuses := make([]*pb.PlanStatusWithID, 0, len(statuses))
	for _, status := range statuses {
		protoStatuses = append(protoStatuses, status.ToProto())
	}

	return &pb.ListPlanStatusesResponse{PlanStatusesWithIds: protoStatuses}, nil
}

func (server *serviceServer) GetPlan(ctx context.Context, req *pb.GetPlanRequest) (*pb.GetPlanResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	r, err := getPlanRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	planHistory, err := svc.PlanHistory(ctx, r)
	if err != nil {
		return nil, err
	}

	cpws := planHistory[0].ToProto()

	history := []*pb.PlanWithStatus{}
	for _, plan := range planHistory[1:] {
		history = append(history, plan.ToProto())
	}

	return &pb.GetPlanResponse{CurrentPlanWithStatus: cpws, ReplanHistory: history}, nil
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
