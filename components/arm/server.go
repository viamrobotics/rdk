// Package arm contains a gRPC based arm service server.
package arm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

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

// MoveThroughJointPositionsStreamed is the bidi handler for the streamed RPC. It reads the
// mandatory Init message first, locates the arm, then spawns one goroutine that pumps further
// stream messages into a points channel and one that drains the implementation's responses
// channel onto the wire. The implementation runs on the calling goroutine; its returned error
// becomes the terminal gRPC status.
func (s *serviceServer) MoveThroughJointPositionsStreamed(
	stream pb.ArmService_MoveThroughJointPositionsStreamedServer,
) error {
	// Derive a cancellable context from the stream's. We pass this ctx to the impl and watch it
	// in the recv pump, so a Send failure (or the impl's own return) can cancel both the impl
	// and the recv-side `batches <- out` send via a single call.
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	s.logger.CInfow(ctx, "XXX ACM srv: handler entered")
	first, err := stream.Recv()
	if err != nil {
		s.logger.CInfow(ctx, "XXX ACM srv: first stream.Recv error", "err", err)
		return err
	}
	init := first.GetInit()
	if init == nil {
		s.logger.CInfow(ctx, "XXX ACM srv: first message not Init",
			"oneof", fmt.Sprintf("%T", first.GetMessage()))
		return status.Error(codes.InvalidArgument, "first message must be Init")
	}
	name := first.GetName()
	s.logger.CInfow(ctx, "XXX ACM srv: read Init", "name", name)
	a, err := s.coll.Resource(name)
	if err != nil {
		s.logger.CInfow(ctx, "XXX ACM srv: resource lookup failed", "name", name, "err", err)
		return err
	}
	operation.CancelOtherWithLabel(ctx, name)

	// Kinematics is used only to interpret JointPositions on the wire. We tolerate failure here
	// the same way the unary path does (degrees/revolute fallback), so the streamed RPC stays
	// usable on arms whose kinematics haven't been registered yet.
	kinStart := time.Now()
	//nolint:errcheck
	model, _ := a.Kinematics(ctx)
	s.logger.CInfow(ctx, "XXX ACM srv: Kinematics returned",
		"name", name, "elapsed_ms", time.Since(kinStart).Milliseconds(), "model_nil", model == nil)

	batches := make(chan []TrajectoryPoint)
	responses := make(chan Response)

	// Recv pump: stream -> batches. One wire TrajectoryBatch becomes one slice on the channel,
	// preserving the caller-chosen batching for the impl. Closes batches on EOF, wire error, ctx
	// cancel, or invalid message so the impl sees end-of-stream. We do NOT wait on this goroutine
	// before returning: it may be blocked inside stream.Recv() with nothing we can do to unblock
	// it short of returning from the handler, at which point gRPC cancels the stream's underlying
	// context and stream.Recv() unblocks. Recv-side terminal errors are logged but not surfaced;
	// for the PoC, the impl's error is the load-bearing signal.
	utils.PanicCapturingGo(func() {
		defer close(batches)
		s.logger.CInfow(ctx, "XXX ACM srv: recv pump entered", "name", name)
		batchCount := 0
		pointCount := 0
		for {
			req, err := stream.Recv()
			if err != nil {
				s.logger.CInfow(ctx, "XXX ACM srv: recv pump stream.Recv returned",
					"name", name, "err", err,
					"is_eof", errors.Is(err, io.EOF),
					"batches_so_far", batchCount, "points_so_far", pointCount)
				return
			}
			batch := req.GetBatch()
			if batch == nil {
				s.logger.CInfow(ctx, "XXX ACM srv: recv pump got non-batch oneof",
					"name", name,
					"oneof", fmt.Sprintf("%T", req.GetMessage()),
					"batches_so_far", batchCount, "points_so_far", pointCount)
				return
			}
			pbPoints := batch.GetPoints()
			out := make([]TrajectoryPoint, 0, len(pbPoints))
			for _, p := range pbPoints {
				tp, err := trajectoryPointFromProto(model, p)
				if err != nil {
					s.logger.CInfow(ctx, "XXX ACM srv: recv pump bad TrajectoryPoint",
						"name", name, "err", err)
					return
				}
				out = append(out, tp)
			}
			batchCount++
			s.logger.CInfow(ctx, "XXX ACM srv: recv pump got batch",
				"name", name, "batch_idx", batchCount, "points_in_batch", len(out))
			select {
			case batches <- out:
				pointCount += len(out)
			case <-ctx.Done():
				s.logger.CInfow(ctx, "XXX ACM srv: recv pump ctx done while pushing",
					"name", name, "batches_so_far", batchCount, "points_so_far", pointCount)
				return
			}
		}
	})

	// Send pump: responses -> stream. Exits when responses is closed (after impl returns) or
	// when a Send fails. On Send failure we cancel the derived context so the impl notices via
	// ctx.Done() and returns promptly, then drain the channel so the impl's writes don't block
	// during shutdown.
	sendDone := make(chan struct{})
	utils.PanicCapturingGo(func() {
		defer close(sendDone)
		respCount := 0
		for resp := range responses {
			_ = resp // TODO(post-PoC): wire fields here when Arm.Response grows beyond {}.
			respCount++
			if err := stream.Send(&pb.MoveThroughJointPositionsStreamedResponse{}); err != nil {
				s.logger.CInfow(ctx, "XXX ACM srv: send pump stream.Send failed",
					"name", name, "err", err, "responses_sent", respCount)
				cancel()
				for range responses {
				}
				return
			}
		}
		s.logger.CInfow(ctx, "XXX ACM srv: send pump exit (responses closed)",
			"name", name, "responses_sent", respCount)
	})

	s.logger.CInfow(ctx, "XXX ACM srv: calling impl", "name", name)
	implStart := time.Now()
	implErr := a.MoveThroughJointPositionsStreamed(ctx, batches, responses, init.GetExtra().AsMap())
	s.logger.CInfow(ctx, "XXX ACM srv: impl returned",
		"name", name, "elapsed_ms", time.Since(implStart).Milliseconds(), "err", implErr)

	// Cancel before closing responses: if the impl returned cleanly, the recv pump may be parked
	// on `batches <- out`; cancel() unblocks the select so the goroutine can wind down.
	cancel()
	close(responses)
	<-sendDone
	s.logger.CInfow(ctx, "XXX ACM srv: handler exiting", "name", name, "err", implErr)

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
