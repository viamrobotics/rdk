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

// StreamArmJointPositions is the bidi handler for the streamed RPC. It reads the Init message
// that has to come first, resolves the motion service, and runs the implementation right here on
// the handler goroutine. A helper goroutine feeds wire batches into the targets channel the
// implementation reads from. There is no response goroutine: the response stream currently
// carries no data, so nothing is ever sent on it.
func (server *serviceServer) StreamArmJointPositions(stream pb.MotionService_StreamArmJointPositionsServer) error {
	// We run the impl under a context we can cancel ourselves, derived from the stream's. That
	// gives us a single lever: cancelling it stops the impl and also unblocks the recv goroutine's
	// `targets <- out` send, whether the trigger was the impl simply returning or a broken stream.
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

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
	opts := streamOptionsFromProto(init.GetOptions())
	extra := init.GetExtra().AsMap()

	targets := make(chan []referenceframe.Input)

	// When the recv side hits something terminal, a stray message or a stream that breaks, that
	// is the error the client should see, not whatever the impl happened to return on its way out.
	// recvErrCh carries it back. It is buffered and we keep only the first write, so the recv
	// goroutine can report and move on without blocking here.
	recvErrCh := make(chan error, 1)
	setRecvErr := func(err error) {
		select {
		case recvErrCh <- err:
		default:
		}
	}

	// This goroutine reads the wire and hands the impl each target one at a time. A clean
	// end-of-stream (the client closing its send, or us cancelling) just closes targets so the
	// impl knows nothing more is coming. Anything else, a stray non-Targets message or a stream
	// that breaks partway, is a fault the client needs to hear about: we stash it, cancel so the
	// impl stops, and let the handler return it in place of whatever the impl says.
	utils.PanicCapturingGo(func() {
		defer close(targets)
		for {
			req, err := stream.Recv()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					setRecvErr(err)
					cancel()
				}
				return
			}
			batch := req.GetTargets()
			if batch == nil {
				setRecvErr(status.Errorf(codes.InvalidArgument, "expected Targets, got %T", req.GetMessage()))
				cancel()
				return
			}
			for _, jp := range batch.GetPositions() {
				select {
				case targets <- jointPositionsFromProto(jp):
				case <-ctx.Done():
					return
				}
			}
		}
	})

	implErr := svc.StreamArmJointPositions(ctx, armName, opts, targets, extra)

	// By now the impl has returned. It may not have drained targets, since it can finish or fault
	// mid-stream, which leaves the recv goroutine parked on `targets <- out` with nobody reading.
	// Cancelling releases that send so the recv goroutine can exit.
	cancel()

	// If the recv side recorded a terminal fault, that is the real reason the stream ended, so we
	// return it ahead of whatever the impl came back with as it unwound.
	select {
	case recvErr := <-recvErrCh:
		return recvErr
	default:
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
