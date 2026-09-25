package motion

import (
	"context"
	"io"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/motion/v1"
	"go.viam.com/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the MotionService from motion.proto.
type serviceServer struct {
	pb.UnimplementedMotionServiceServer
	coll resource.APIResourceGetter[Service]
}

// NewRPCServiceServer constructs a motion gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Service], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

func (server *serviceServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	r, err := MoveReqFromProto(req)
	if err != nil {
		return nil, err
	}
	success, err := svc.Move(ctx, r)
	return &pb.MoveResponse{Success: success}, err
}

func (server *serviceServer) MoveOnMap(ctx context.Context, req *pb.MoveOnMapRequest) (*pb.MoveOnMapResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	r, err := moveOnMapRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	id, err := svc.MoveOnMap(ctx, r)
	if err != nil {
		return nil, err
	}

	return &pb.MoveOnMapResponse{ExecutionId: id.String()}, nil
}

func (server *serviceServer) MoveOnGlobe(ctx context.Context, req *pb.MoveOnGlobeRequest) (*pb.MoveOnGlobeResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	r, err := moveOnGlobeRequestFromProto(req)
	if err != nil {
		return nil, err
	}

	id, err := svc.MoveOnGlobe(ctx, r)
	if err != nil {
		return nil, err
	}

	return &pb.MoveOnGlobeResponse{ExecutionId: id.String()}, nil
}

// This is preserving backwards compatibility for older updated clients.
//
//nolint:staticcheck
func (server *serviceServer) GetPose(ctx context.Context, req *pb.GetPoseRequest) (*pb.GetPoseResponse, error) {
	svc, err := server.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	if req.ComponentName == "" {
		return nil, errors.New("must provide component name")
	}
	transforms, err := referenceframe.LinkInFramesFromTransformsProtobuf(req.GetSupplementalTransforms())
	if err != nil {
		return nil, err
	}
	pose, err := svc.GetPose(ctx, req.ComponentName, req.DestinationFrame, transforms, req.Extra.AsMap())
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

	componentName := req.GetComponentName()
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

func (server *serviceServer) TempStreamArmJointPositions(stream pb.MotionService_TempStreamArmJointPositionsServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Receive the first message, which must be an Init message, and which contains
	// the service whose impl to call and the options with which to call it.
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	init := first.GetInit()
	if init == nil {
		return status.Error(codes.InvalidArgument, "first message must be Init")
	}
	svc, err := server.coll.Resource(first.GetName())
	if err != nil {
		return err
	}
	armName := init.GetComponentName()
	if armName == "" {
		return status.Error(codes.InvalidArgument, "Init.component_name is required")
	}
	opts := tempStreamOptionsFromProto(init.GetOptions())
	extra := init.GetExtra().AsMap()

	targetsCh := make(chan []referenceframe.Input)
	responsesCh := make(chan TempStreamResponse)

	// "Recv goroutine": stream.Recv()'s targets from the client and sends them to the impl.
	//
	// Whenever the client is idle, this goroutine is parked in stream.Recv, which only the client
	// acting or this handler returning can wake. So it outlives this handler by design.
	recvResult := make(chan error, 1)
	utils.PanicCapturingGo(func() {
		// Every exit below overwrites err; if none did, the goroutine panicked.
		err := errors.New("motion streaming server recv goroutine panicked")
		defer func() {
			recvResult <- err
			if err != nil {
				cancel()
			}
		}()

		for {
			var req *pb.TempStreamArmJointPositionsRequest
			if req, err = stream.Recv(); err != nil {
				// io.EOF from Recv means the client closed its send side. Close the targetsCh to
				// propagate clean end of stream to the impl.
				if errors.Is(err, io.EOF) {
					err = nil
					close(targetsCh)
				}
				return
			}
			targets := req.GetTargets()
			if targets == nil {
				err = status.Errorf(codes.InvalidArgument, "expected Targets, got %T", req.GetMessage())
				return
			}
			for _, jps := range targets.GetPositions() {
				select {
				case targetsCh <- referenceframe.JointPositionsToRadians(jps):
				case <-ctx.Done():
					// Do not return an error from this recv goroutine. If the client canceled, the
					// client has gone away and will never see the error anyway. If send goroutine or
					// handler canceled the context, let them report their error.
					err = nil
					return
				}
			}
		}
	})

	// "Send goroutine": receives responses from the impl and stream.Send()'s them to the client.
	sendResult := make(chan error, 1)
	utils.PanicCapturingGo(func() {
		err := errors.New("motion streaming server send goroutine panicked")
		defer func() {
			if err != nil {
				cancel()
			}
			sendResult <- err
		}()

		for resp := range responsesCh {
			_ = resp // TempStreamResponse carries no fields yet.
			if err = stream.Send(&pb.TempStreamArmJointPositionsResponse{}); err != nil {
				return
			}
		}
		err = nil
	})

	implErr := svc.TempStreamArmJointPositions(ctx, armName, opts, targetsCh, responsesCh, extra)

	// Release the send goroutine.
	close(responsesCh)

	// Get the recv goroutine's ending error, if it has set one.
	var recvErr error
	select {
	case recvErr = <-recvResult:
	default:
	}

	// Get the send goroutine's ending error.
	sendErr := <-sendResult

	// The recv and send goroutines only end with an error when they are the cause,
	// so between the two, it doesn't matter which one we prefer.
	if recvErr != nil {
		return recvErr
	}
	if sendErr != nil {
		return sendErr
	}
	return implErr
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

// GetStatus returns the status of the motion service.
func (server *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := server.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.GetStatusFromResourceServer(ctx, res, req)
}
