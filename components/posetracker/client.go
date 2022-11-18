package posetracker

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/posetracker/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/referenceframe"
)

// client implements PoseTrackerServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.PoseTrackerServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) PoseTracker {
	c := pb.NewPoseTrackerServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) Poses(
	ctx context.Context, bodyNames []string, extra map[string]interface{},
) (BodyToPoseInFrame, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	req := &pb.GetPosesRequest{
		Name:      c.name,
		BodyNames: bodyNames,
		Extra:     ext,
	}
	resp, err := c.client.GetPoses(ctx, req)
	if err != nil {
		return nil, err
	}
	result := BodyToPoseInFrame{}
	for key, pf := range resp.GetBodyPoses() {
		result[key] = referenceframe.ProtobufToPoseInFrame(pf)
	}
	return result, nil
}

func (c *client) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return Readings(ctx, c)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
