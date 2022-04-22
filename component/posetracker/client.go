package posetracker

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/grpc"
	pbgeneric "go.viam.com/rdk/proto/api/component/generic/v1"
	pb "go.viam.com/rdk/proto/api/component/posetracker/v1"
	"go.viam.com/rdk/referenceframe"
)

// serviceClient is a client that satisfies the pose_tracker.proto contract.
type serviceClient struct {
	conn    rpc.ClientConn
	client  pb.PoseTrackerServiceClient
	gclient pbgeneric.GenericServiceClient
	logger  golog.Logger
}

// newServiceClient returns a new serviceClient served at the given address.
func newServiceClient(ctx context.Context, address string, logger golog.Logger, opts ...rpc.DialOption) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewPoseTrackerServiceClient(conn)
	gClient := pbgeneric.NewGenericServiceClient(conn)
	sc := &serviceClient{
		conn:    conn,
		client:  client,
		gclient: gClient,
		logger:  logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is a pose tracker client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(
	ctx context.Context, name string, address string,
	logger golog.Logger, opts ...rpc.DialOption,
) (PoseTracker, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
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
	command, err := structpb.NewStruct(cmd)
	if err != nil {
		return nil, err
	}

	resp, err := c.gclient.Do(ctx, &pbgeneric.DoRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
