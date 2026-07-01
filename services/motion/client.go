package motion

import (
	"context"

	"github.com/google/uuid"
	robotpb "go.viam.com/api/robot/v1"
	pb "go.viam.com/api/service/motion/v1"
	vprotoutils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"braces.dev/errtrace"
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
		return false, errtrace.Wrap(err)
	}
	resp, err := c.client.Move(ctx, protoReq)
	if err != nil {
		return false, errtrace.Wrap(err)
	}
	return resp.Success, nil
}

func (c *client) MoveOnMap(ctx context.Context, req MoveOnMapReq) (ExecutionID, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
	}

	resp, err := c.client.MoveOnMap(ctx, protoReq)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
	}

	executionID, err := uuid.Parse(resp.ExecutionId)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
	}

	return executionID, nil
}

func (c *client) MoveOnGlobe(
	ctx context.Context,
	req MoveOnGlobeReq,
) (ExecutionID, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
	}

	resp, err := c.client.MoveOnGlobe(ctx, protoReq)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
	}

	executionID, err := uuid.Parse(resp.ExecutionId)
	if err != nil {
		return uuid.Nil, errtrace.Wrap(err)
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
		return nil, errtrace.Wrap(err)
	}
	transforms, err := referenceframe.LinkInFramesToTransformsProtobuf(supplementalTransforms)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	resp, err := c.robotClient.GetPose(ctx, &robotpb.GetPoseRequest{
		ComponentName:          componentName,
		DestinationFrame:       destinationFrame,
		SupplementalTransforms: transforms,
		Extra:                  ext,
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return referenceframe.ProtobufToPoseInFrame(resp.Pose), nil
}

func (c *client) StopPlan(ctx context.Context, req StopPlanReq) error {
	ext, err := vprotoutils.StructToStructPb(req.Extra)
	if err != nil {
		return errtrace.Wrap(err)
	}
	_, err = c.client.StopPlan(ctx, &pb.StopPlanRequest{
		Name:          c.name,
		ComponentName: req.ComponentName,
		Extra:         ext,
	})
	return errtrace.Wrap(err)
}

func (c *client) ListPlanStatuses(ctx context.Context, req ListPlanStatusesReq) ([]PlanStatusWithID, error) {
	ext, err := vprotoutils.StructToStructPb(req.Extra)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	resp, err := c.client.ListPlanStatuses(ctx, &pb.ListPlanStatusesRequest{
		Name:            c.name,
		OnlyActivePlans: req.OnlyActivePlans,
		Extra:           ext,
	})
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	pswids := make([]PlanStatusWithID, 0, len(resp.PlanStatusesWithIds))
	for _, status := range resp.PlanStatusesWithIds {
		pswid, err := planStatusWithIDFromProto(status)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}

		pswids = append(pswids, pswid)
	}
	return pswids, errtrace.Wrap(err)
}

func (c *client) PlanHistory(
	ctx context.Context,
	req PlanHistoryReq,
) ([]PlanWithStatus, error) {
	protoReq, err := req.toProto(c.name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	resp, err := c.client.GetPlan(ctx, protoReq)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	statusHistory := make([]PlanWithStatus, 0, len(resp.ReplanHistory))
	for _, status := range resp.ReplanHistory {
		s, err := planWithStatusFromProto(status)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}
		statusHistory = append(statusHistory, s)
	}
	pws, err := planWithStatusFromProto(resp.CurrentPlanWithStatus)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return append([]PlanWithStatus{pws}, statusHistory...), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return errtrace.Wrap2(protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd))
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return errtrace.Wrap2(protoutils.GetStatusFromResourceClient(ctx, c.client, c.name))
}
