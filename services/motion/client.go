package motion

import (
	"context"
	"errors"
	"io"
	"sync"

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

// StreamArmJointPositions implements motion.Service over the StreamArmJointPositions bidi RPC.
func (c *client) StreamArmJointPositions(
	ctx context.Context,
	armName string,
	opts StreamOptions,
	targets <-chan []referenceframe.Input,
	extra map[string]interface{},
) error {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	// We open the stream under a context we can cancel, so one cancel() both tears the gRPC
	// stream down and tells the send goroutine to quit. We lean on that when the recv loop
	// finishes: without it, the send goroutine could sit forever waiting on a caller who never
	// closes targets.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := c.client.StreamArmJointPositions(ctx)
	if err != nil {
		return err
	}

	if err := stream.Send(&pb.StreamArmJointPositionsRequest{
		Name: c.name,
		Message: &pb.StreamArmJointPositionsRequest_Init_{
			Init: &pb.StreamArmJointPositionsRequest_Init{
				ComponentName: armName,
				Options:       streamOptionsToProto(opts),
				Extra:         ext,
			},
		},
	}); err != nil {
		return err
	}

	// Feed the caller's targets onto the wire, one Targets message per waypoint. Batching several
	// waypoints per wire message is a wire-efficiency optimization that can be added later if
	// needed; this keeps the translation simple for now.
	var sendErr error
	var sendOnce sync.Once
	setSendErr := func(e error) { sendOnce.Do(func() { sendErr = e }) }
	sendDone := make(chan struct{})
	goutils.PanicCapturingGo(func() {
		defer close(sendDone)
		for {
			select {
			case <-ctx.Done():
				setSendErr(ctx.Err())
				return
			case t, ok := <-targets:
				if !ok {
					if err := stream.CloseSend(); err != nil {
						setSendErr(err)
					}
					return
				}
				if err := stream.Send(&pb.StreamArmJointPositionsRequest{
					Message: &pb.StreamArmJointPositionsRequest_Targets_{
						Targets: &pb.StreamArmJointPositionsRequest_Targets{
							Positions: []*armpb.JointPositions{jointPositionsToProto(t)},
						},
					},
				}); err != nil {
					setSendErr(err)
					return
				}
			}
		}
	})

	// Back on the calling goroutine, read the response stream to completion. The responses carry
	// no data; we read only to learn the RPC's terminal status, which arrives on the next Recv()
	// after the server ends the stream.
	var recvErr error
	for {
		if _, err := stream.Recv(); err != nil {
			if !errors.Is(err, io.EOF) {
				recvErr = err
			}
			break
		}
	}
	// Tear the stream down and wake the send goroutine, which may still be parked waiting on
	// targets.
	cancel()
	<-sendDone

	if recvErr != nil {
		return recvErr
	}
	return sendErr
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return protoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}
