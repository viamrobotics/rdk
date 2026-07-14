// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"
	"errors"
	"io"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/arm/v1"
	"go.viam.com/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

const unimplemented = "unimplemented"

// serviceServer implements the ArmService from arm.proto.
type serviceServer struct {
	pb.UnimplementedArmServiceServer
	coll resource.APIResourceGetter[Arm]

	logger logging.Logger
}

// NewRPCServiceServer constructs an arm gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Arm], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll, logger: logger.Sublogger("arm_server")}
}

// GetEndPosition returns the position of the arm specified.
func (s *serviceServer) GetEndPosition(
	ctx context.Context,
	req *pb.GetEndPositionRequest,
) (*pb.GetEndPositionResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.EndPosition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	// Return a default empty value if the position returned is nil,
	// this guards against nil objects being transferred over the wire.
	if pos == nil {
		pose := &commonpb.Pose{}
		return &pb.GetEndPositionResponse{Pose: pose}, nil
	}

	return &pb.GetEndPositionResponse{Pose: spatialmath.PoseToProtobuf(pos)}, nil
}

// GetJointPositions gets the current joint position of an arm of the underlying robot.
func (s *serviceServer) GetJointPositions(ctx context.Context, req *pb.GetJointPositionsRequest) (*pb.GetJointPositionsResponse, error) {
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := arm.JointPositions(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	jp, err := referenceframe.JointPositionsFromInputs(m, pos)
	if err != nil {
		return nil, err
	}
	return &pb.GetJointPositionsResponse{Positions: jp}, nil
}

// MoveToPosition returns the position of the arm specified.
func (s *serviceServer) MoveToPosition(ctx context.Context, req *pb.MoveToPositionRequest) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	s.logger.Debugw("Move to position", "res", req.Name, "getTo", req.GetTo())
	return &pb.MoveToPositionResponse{}, arm.MoveToPosition(
		ctx,
		spatialmath.NewPoseFromProtobuf(req.GetTo()),
		req.Extra.AsMap(),
	)
}

// MoveToJointPositions moves an arm of the underlying robot to the requested joint positions.
func (s *serviceServer) MoveToJointPositions(
	ctx context.Context,
	req *pb.MoveToJointPositionsRequest,
) (*pb.MoveToJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	s.logger.Debugw("Move to joint positions", "res", req.Name, "pos", req.Positions)
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	inputs, err := referenceframe.InputsFromJointPositions(m, req.Positions)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToJointPositionsResponse{}, arm.MoveToJointPositions(ctx, inputs, req.Extra.AsMap())
}

// MoveThroughJointPositions moves an arm of the underlying robot through the requested joint positions.
func (s *serviceServer) MoveThroughJointPositions(
	ctx context.Context,
	req *pb.MoveThroughJointPositionsRequest,
) (*pb.MoveThroughJointPositionsResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	s.logger.Debugw("Move through joint positions", "res", req.Name, "pos", req.Positions)
	// safe to ignore error because conversion function below can handle nil values and warning messages are logged from client
	//nolint:errcheck
	m, _ := arm.Kinematics(ctx)
	allInputs := make([][]referenceframe.Input, 0, len(req.Positions))
	for _, position := range req.Positions {
		inputs, err := referenceframe.InputsFromJointPositions(m, position)
		if err != nil {
			return nil, err
		}
		allInputs = append(allInputs, inputs)
	}
	err = arm.MoveThroughJointPositions(ctx, allInputs, moveOptionsFromProtobuf(req.Options), req.Extra.AsMap())
	return &pb.MoveThroughJointPositionsResponse{}, err
}

// MoveThroughJointPositionsStreamed is the bidi handler for the streamed RPC. It reads the Init
// message that has to come first, finds the arm, and runs the implementation right here on the
// handler goroutine. Two helper goroutines bracket that call: one feeds wire batches in, the other
// carries the implementation's responses back out to the client. Whatever the implementation
// returns becomes the terminal gRPC status.
func (s *serviceServer) MoveThroughJointPositionsStreamed(
	stream pb.ArmService_MoveThroughJointPositionsStreamedServer,
) error {
	// We run the impl under a context we can cancel ourselves, derived from the stream's. That gives
	// us a single lever: cancelling it stops the impl and also unblocks the recv goroutine's
	// `batches <- out` send, whether the trigger was a failed Send or the impl simply returning.
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
	name := first.GetName()
	s.logger.Debugw("Move through joint positions streamed", "res", name)
	a, err := s.coll.Resource(name)
	if err != nil {
		return err
	}
	operation.CancelOtherWithLabel(ctx, name)

	// Kinematics is used only to interpret JointPositions on the wire. We tolerate failure here
	// the same way the unary path does (degrees/revolute fallback), so the streamed RPC stays
	// usable on arms whose kinematics haven't been registered yet.
	//nolint:errcheck
	model, _ := a.Kinematics(ctx)

	batches := make(chan []TrajectoryPoint)
	responses := make(chan Response)

	// When the recv side hits something terminal, a bad point, a stray message, a stream that breaks,
	// that is the error the client should see, not whatever the impl happened to return on its way
	// out. recvErrCh carries it back. It is buffered and we keep only the first write, so the recv
	// goroutine can report and move on without blocking here.
	recvErrCh := make(chan error, 1)
	setRecvErr := func(err error) {
		select {
		case recvErrCh <- err:
		default:
		}
	}

	// This goroutine reads the wire and hands the impl each batch as one slice, so the impl sees the
	// same batching the caller chose. A clean end-of-stream (the client closing its send, or us
	// cancelling) just closes batches so the impl knows nothing more is coming. Anything else, a
	// malformed point, a stray non-batch message, a stream that breaks partway, is a fault the client
	// needs to hear about: we stash it, cancel so the impl stops, and let the handler return it in
	// place of whatever the impl says. Either way batches is closed on the way out. We never wait for
	// this goroutine, though: it is usually parked in stream.Recv(), and the only thing that wakes it
	// is the handler returning and gRPC tearing the stream down.
	utils.PanicCapturingGo(func() {
		defer close(batches)
		for {
			req, err := stream.Recv()
			if err != nil {
				// EOF is the client's clean end-of-stream; anything else is a broken stream that
				// the client must hear about.
				if !errors.Is(err, io.EOF) {
					setRecvErr(err)
					cancel()
				}
				return
			}
			batch := req.GetBatch()
			if batch == nil {
				// After Init, every message must be a TrajectoryBatch.
				setRecvErr(status.Errorf(codes.InvalidArgument, "expected TrajectoryBatch, got %T", req.GetMessage()))
				cancel()
				return
			}
			pbPoints := batch.GetPoints()
			out := make([]TrajectoryPoint, 0, len(pbPoints))
			for _, p := range pbPoints {
				tp, err := trajectoryPointFromProto(model, p)
				if err != nil {
					setRecvErr(status.Errorf(codes.InvalidArgument, "invalid trajectory point: %v", err))
					cancel()
					return
				}
				out = append(out, tp)
			}
			select {
			case batches <- out:
			case <-ctx.Done():
				return
			}
		}
	})

	// This goroutine carries the impl's responses out to the client. It stops when the impl is done
	// (responses closed) or when a Send fails. On a failed Send we cancel, so the impl learns through
	// ctx.Done() that there is no point continuing.
	sendDone := make(chan struct{})
	utils.PanicCapturingGo(func() {
		defer close(sendDone)
		for resp := range responses {
			_ = resp // Response carries no fields yet; future per-batch status will be written onto the send here.
			if err := stream.Send(&pb.MoveThroughJointPositionsStreamedResponse{}); err != nil {
				cancel()
				// Keep draining responses until the handler closes it. This is not defensiveness
				// against a bad impl, it is what the contract asks for: an impl may write responses and
				// return without ever watching ctx ("write responses, return when done"). After a
				// failed Send, an impl like that would wedge on its next write if we stopped reading,
				// and never return. Draining keeps it moving until it sees batches close and returns on
				// its own.
				for range responses {
				}
				return
			}
		}
	})

	implErr := a.MoveThroughJointPositionsStreamed(ctx, batches, responses, init.GetExtra().AsMap())

	// By now the impl has returned. It may not have drained batches, since it can finish or fault
	// mid-stream, which leaves the recv goroutine parked on `batches <- out` with nobody reading.
	// Cancelling releases that send so the recv goroutine can exit; closing responses lets the send
	// goroutine finish, and we wait for it. Order does not matter here: the two calls poke two
	// different goroutines.
	cancel()
	close(responses)
	<-sendDone

	// If the recv side recorded a terminal fault, that is the real reason the stream ended, so we
	// return it ahead of whatever the impl came back with as it unwound.
	select {
	case recvErr := <-recvErrCh:
		return recvErr
	default:
	}

	return implErr
}

// Stop stops the arm specified.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	arm, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, arm.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := arm.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		// if the error tells us the method is unimplemented, then we
		// can use the kinematics and joint positions endpoints to
		// construct the geometries of the arm
		if strings.Contains(err.Error(), unimplemented) {
			kinematicsPbResp, err := s.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			model, err := referenceframe.KinematicModelFromProtobuf(req.GetName(), kinematicsPbResp)
			if err != nil {
				return nil, err
			}

			jointPbResp, err := s.GetJointPositions(ctx, &pb.GetJointPositionsRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			jointPositionsPb := jointPbResp.GetPositions()

			// Joint positions are in degrees but model.Geometries expects radians, so we convert them here.
			jointPositionsRads, err := referenceframe.InputsFromJointPositions(model, jointPositionsPb)
			if err != nil {
				return nil, err
			}
			gifs, err := model.Geometries(jointPositionsRads)
			if err != nil {
				return nil, err
			}
			return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(
				gifs.Geometries())}, nil
		}
		return nil, err
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

// Get3DModels returns the 3D models of the arm.
func (s *serviceServer) Get3DModels(ctx context.Context, req *commonpb.Get3DModelsRequest) (*commonpb.Get3DModelsResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	models, err := arm.Get3DModels(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &commonpb.Get3DModelsResponse{Models: models}, nil
}

// GetKinematics returns the kinematics information associated with the arm.
func (s *serviceServer) GetKinematics(ctx context.Context, req *commonpb.GetKinematicsRequest) (*commonpb.GetKinematicsResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	model, err := arm.Kinematics(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.KinematicModelToProtobuf(model), nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	arm, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	s.logger.Debugw("DoCommand", "res", req.Name, "req", req)
	return protoutils.DoFromResourceServer(ctx, arm, req)
}

// GetStatus returns the status of the arm.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.GetStatusFromResourceServer(ctx, res, req)
}
