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

// TempStreamArmJointPositions serves one streaming session for an arm.
//
// The first message must be Init; it names the motion service to use, the arm, and the session
// options. Every message after it must be Targets.
//
// This handler completes cleanly if the client closes its send side (CloseSend), and none of the
// errors below occurs. In that case, this handler signals the impl to complete cleanly, then
// waits for the impl to return; the impl drains all targets to the arm and waits for the arm to
// finish executing them and come to a stop.
//
// The following classes of error can occur:
//
// "Invalid Init":
// The first message is not Init, names a motion service that does not exist, omits the arm's
// component name, or the stream ends before an Init arrives. The handler returns an error without
// calling the impl; no arm motion has been commanded, so there is nothing to stop.
//
// "Something above the impl causes the stream to abort":
// This encompasses various errors such as stream.Recv returning an error besides io.EOF (including
// if the client cancels or the connection is lost) or an unexpected message, or stream.Send returning
// an error. The handler cancels the impl's context and waits until the impl returns; the impl stops
// processing the targets, stops the arm, and returns once the arm has stopped.
//
// "The impl errors":
// If the impl fails on its own, the impl stops the arm and waits until the arm has stopped. The
// handler waits until the impl returns, then returns the impl's error.
//
// If more than one of these happen, this handler returns the error detected above the impl,
// because the impl's own error is usually only its echo.
func (server *serviceServer) TempStreamArmJointPositions(stream pb.MotionService_TempStreamArmJointPositionsServer) (retErr error) {
	ctx, cancel := context.WithCancelCause(stream.Context())
	defer cancel(nil)

	// Receive the first message, which must be an Init message.
	// This is done outside the recv goroutine, because the Init message contains
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

	// Per grpc-go's ServerStream contract (https://pkg.go.dev/google.golang.org/grpc#ServerStream),
	// this handler writes messages to the client via Send, but sets an error status on the call by
	// returning an error. (The client receives both via its Recv.) This handler's recv and send
	// goroutines cancel the context with a cause, so that the handler can return the original cause. An
	// error from either goroutine takes precedence over an error from the impl, because usually the
	// cancel is what made the impl return, so the impl's error is only its echo. In the rare race where
	// the impl finished or failed on its own at the same time, the cause still names something that ended
	// the call.

	// "recv goroutine": stream.Recv()'s targets from the client and sends them to the impl.
	utils.PanicCapturingGo(func() {
		// Every exit below overwrites err; if none did, the goroutine panicked.
		err := errors.New("motion streaming server recv goroutine panicked")
		defer func() {
			if err != nil {
				cancel(err)
			}
		}()

		for {
			// A successful Recv must not touch err, so that the panic sentinel survives a panic below.
			req, recvErr := stream.Recv()
			if recvErr != nil {
				err = recvErr
				// io.EOF from Recv means the client closed its send side. Close targetsCh to give the
				// impl a clean end of input.
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
					// Whoever canceled already set the cause.
					err = nil
					return
				}
			}
		}
	})

	// "send goroutine": receives responses from the impl and stream.Send()'s them to the client.
	sendDone := make(chan struct{})
	utils.PanicCapturingGo(func() {
		err := errors.New("motion streaming server send goroutine panicked")
		defer func() {
			if err != nil {
				cancel(err)
			}
			close(sendDone)
		}()

		for resp := range responsesCh {
			if err = stream.Send(tempStreamResponseToProto(resp)); err != nil {
				return
			}
		}
		err = nil
	})

	// Release the send goroutine and wait for it, so that nothing Sends after this handler returns,
	// then pick the return value. This is deferred so that it also runs if the impl panics: the rpc
	// server recovers handler panics, and without this the send goroutine would be left blocked on
	// responsesCh forever. The recv goroutine is not waited for: it may be parked in Recv, which
	// only the client acting or this handler returning can wake, so it exits on its own after we
	// return, with its error (if any) already delivered as the cancel cause.
	var implErr error
	defer func() {
		close(responsesCh)
		<-sendDone

		if cause := context.Cause(ctx); cause != nil {
			retErr = cause
			return
		}
		retErr = implErr
	}()

	implErr = svc.TempStreamArmJointPositions(ctx, armName, opts, targetsCh, responsesCh, extra)
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
