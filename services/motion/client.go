package motion

import (
	"context"

	"github.com/google/uuid"
	pb "go.viam.com/api/service/motion/v1"
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
	name   string
	client pb.MotionServiceClient
	logger logging.Logger
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
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) Move(
	ctx context.Context,
	componentName resource.Name,
	destination *referenceframe.PoseInFrame,
	worldState *referenceframe.WorldState,
	constraints *pb.Constraints,
	extra map[string]interface{},
) (bool, error) {
	ext, err := vprotoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	worldStateMsg, err := worldState.ToProtobuf()
	if err != nil {
		return false, err
	}
	resp, err := c.client.Move(ctx, &pb.MoveRequest{
		Name:          c.name,
		ComponentName: protoutils.ResourceNameToProto(componentName),
		Destination:   referenceframe.PoseInFrameToProtobuf(destination),
		WorldState:    worldStateMsg,
		Constraints:   constraints,
		Extra:         ext,
	})
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
	componentName resource.Name,
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
	resp, err := c.client.GetPose(ctx, &pb.GetPoseRequest{
		Name:                   c.name,
		ComponentName:          protoutils.ResourceNameToProto(componentName),
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
		ComponentName: protoutils.ResourceNameToProto(req.ComponentName),
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

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
