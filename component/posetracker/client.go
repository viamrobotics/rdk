package posetracker

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/posetracker/v1"
	"go.viam.com/rdk/referenceframe"
)

// serviceClient is a client that satisfies the pose_tracker.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.PoseTrackerServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewPoseTrackerServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return nil
}

// client is a pose tracker client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) PoseTracker {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) PoseTracker {
	return &client{sc, name}
}

func (c *client) GetPoses(
	ctx context.Context, bodyNames []string,
) (BodyToPoseInFrame, error) {
	req := &pb.GetPosesRequest{
		Name:      c.name,
		BodyNames: bodyNames,
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

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
