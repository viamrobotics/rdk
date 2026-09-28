package motion

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"
	armpb "go.viam.com/api/component/arm/v1"
	robotpb "go.viam.com/api/robot/v1"
	pb "go.viam.com/api/service/motion/v1"
	goutils "go.viam.com/utils"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// client implements MotionServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name        string
	client      pb.MotionServiceClient
	robotClient robotpb.RobotServiceClient
	logger      logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewMotionServiceClient(conn)
	c := &client{
		Named:       name.PrependRemote(remoteName).AsNamed(),
		name:        name.Name,
		client:      grpcClient,
		robotClient: robotpb.NewRobotServiceClient(conn),
		logger:      logger,
	}
	return c, nil
}

func (c *client) Move(ctx context.Context, req MoveReq) (bool, error) {
	protoReq, err := req.ToProto(c.name)
	if err != nil {
		return false, err
	}
	resp, err := c.client.Move(ctx, protoReq)
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}

func (c *client) MoveOnMap(ctx context.Context, req MoveOnMapReq) (ExecutionID, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return uuid.Nil, err
	}

	resp, err := c.client.MoveOnMap(ctx, protoReq)
	if err != nil {
		return uuid.Nil, err
	}

	executionID, err := uuid.Parse(resp.ExecutionId)
	if err != nil {
		return uuid.Nil, err
	}

	return executionID, nil
}

func (c *client) MoveOnGlobe(
	ctx context.Context,
	req MoveOnGlobeReq,
) (ExecutionID, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return uuid.Nil, err
	}

	resp, err := c.client.MoveOnGlobe(ctx, protoReq)
	if err != nil {
		return uuid.Nil, err
	}

	executionID, err := uuid.Parse(resp.ExecutionId)
	if err != nil {
		return uuid.Nil, err
	}

	return executionID, nil
}

func (c *client) GetPose(
	ctx context.Context,
	componentName string,
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

	resp, err := c.robotClient.GetPose(ctx, &robotpb.GetPoseRequest{
		ComponentName:          componentName,
		DestinationFrame:       destinationFrame,
		SupplementalTransforms: transforms,
		Extra:                  ext,
	})
	if err != nil {
		return nil, err
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

func (c *client) StopPlan(ctx context.Context, req StopPlanReq) error {
	ext, err := vprotoutils.StructToStructPb(req.Extra)
	if err != nil {
		return err
	}
	_, err = c.client.StopPlan(ctx, &pb.StopPlanRequest{
		Name:          c.name,
		ComponentName: req.ComponentName,
		Extra:         ext,
	})
	return err
}

func (c *client) ListPlanStatuses(ctx context.Context, req ListPlanStatusesReq) ([]PlanStatusWithID, error) {
	ext, err := vprotoutils.StructToStructPb(req.Extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListPlanStatuses(ctx, &pb.ListPlanStatusesRequest{
		Name:            c.name,
		OnlyActivePlans: req.OnlyActivePlans,
		Extra:           ext,
	})
	if err != nil {
		return nil, err
	}
	pswids := make([]PlanStatusWithID, 0, len(resp.PlanStatusesWithIds))
	for _, status := range resp.PlanStatusesWithIds {
		pswid, err := planStatusWithIDFromProto(status)
		if err != nil {
			return nil, err
		}

		pswids = append(pswids, pswid)
	}
	return pswids, err
}

func (c *client) PlanHistory(
	ctx context.Context,
	req PlanHistoryReq,
) ([]PlanWithStatus, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetPlan(ctx, protoReq)
	if err != nil {
		return nil, err
	}
	statusHistory := make([]PlanWithStatus, 0, len(resp.ReplanHistory))
	for _, status := range resp.ReplanHistory {
		s, err := planWithStatusFromProto(status)
		if err != nil {
			return nil, err
		}
		statusHistory = append(statusHistory, s)
	}
	pws, err := planWithStatusFromProto(resp.CurrentPlanWithStatus)
	if err != nil {
		return nil, err
	}
	return append([]PlanWithStatus{pws}, statusHistory...), nil
}

// TempStreamArmJointPositions drives a gRPC bidi stream, following grpc-go's ClientStream contract
// (https://pkg.go.dev/google.golang.org/grpc#ClientStream).
//
// The server's status is the single source of truth for how the call ended, and it reaches the
// client only through Recv. So the recv goroutine's error is the default result. The exception is
// an error that happens inside this client when trying to send a message, i.e. a message that
// fails to marshal or exceeds the max send size. The server never learns of those, so grpc-go
// returns them directly from Send instead of via Recv, and our send goroutine reports them.
func (c *client) TempStreamArmJointPositions(
	ctx context.Context,
	armName string,
	opts TempStreamOptions,
	targets <-chan []referenceframe.Input,
	responses chan<- TempStreamResponse,
	extra map[string]interface{},
) error {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Opens the HTTP/2 stream to the server; does not send any messages yet.
	stream, err := c.client.TempStreamArmJointPositions(ctx)
	if err != nil {
		return err
	}

	// "send goroutine": receives targets from the client; stream.Send()'s them to the server.
	sendResult := make(chan error, 1)
	goutils.PanicCapturingGo(func() {
		// Every exit below overwrites err; if none did, the goroutine panicked.
		err := errors.New("motion streaming client send goroutine panicked")
		defer func() {
			if err != nil {
				cancel()
			}
			sendResult <- err
		}()

		// Send the initial Init message.
		if err = stream.Send(&pb.TempStreamArmJointPositionsRequest{
			Name: c.name,
			Message: &pb.TempStreamArmJointPositionsRequest_Init_{
				Init: &pb.TempStreamArmJointPositionsRequest_Init{
					ComponentName: armName,
					Options:       tempStreamOptionsToProto(opts),
					Extra:         ext,
				},
			},
		}); err != nil {
			// io.EOF from Send means the stream had already ended.
			// Do not return an error from this send goroutine; the recv side will have the stream's status.
			if errors.Is(err, io.EOF) {
				err = nil
			}
			return
		}

		for {
			select {
			case t, ok := <-targets:
				if !ok {
					// CloseSend always returns nil.
					// Do not return an error from this send goroutine; the recv side will have the stream's status.
					//nolint:errcheck
					stream.CloseSend()
					err = nil
					return
				}
				if err = stream.Send(&pb.TempStreamArmJointPositionsRequest{
					Message: &pb.TempStreamArmJointPositionsRequest_Targets_{
						Targets: &pb.TempStreamArmJointPositionsRequest_Targets{
							Positions: []*armpb.JointPositions{referenceframe.JointPositionsFromRadians(t)},
						},
					},
				}); err != nil {
					// io.EOF from Send means the stream had already ended.
					// Do not return an error from this send goroutine; the recv side will have the stream's status.
					if errors.Is(err, io.EOF) {
						err = nil
					}
					return
				}
			case <-ctx.Done():
				// Either the parent ctx was canceled or the recv goroutine ended the stream.
				// Since the recv goroutine knows which occurred, let it report the error.
				err = nil
				return
			}
		}
	})

	// "recv goroutine": stream.Recv()'s responses from the server and sends them to the client.
	recvResult := make(chan error, 1)
	goutils.PanicCapturingGo(func() {
		err := errors.New("motion streaming client recv goroutine panicked")
		defer func() {
			// Every exit of this goroutine means the stream is done, so alert the send goroutine.
			cancel()
			recvResult <- err
		}()
		for {
			// TempStreamResponse carries no fields yet, so the message itself is not read.
			if _, err = stream.Recv(); err != nil {
				// io.EOF from Recv means the stream ended cleanly.
				if errors.Is(err, io.EOF) {
					err = nil
				} else if ctxErr := ctx.Err(); ctxErr != nil {
					// An error from Recv after ctx is done is just the wire's echo of that cancellation.
					// Surface the ctx error so callers can errors.Is on it.
					err = ctxErr
				}
				return
			}
			select {
			case responses <- TempStreamResponse{}:
			case <-ctx.Done():
				err = ctx.Err()
				return
			}
		}
	})

	// Get the goroutines' ending errors.
	sendErr := <-sendResult
	recvErr := <-recvResult

	// Prefer the send goroutine's error, since it only ends with an error when the error originated there.
	if sendErr != nil {
		return sendErr
	}
	return recvErr
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return protoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}
