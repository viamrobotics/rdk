package objectdetection

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/objectdetection/v1"
)

// client is a client that implements the Object Detection Service.
type client struct {
	conn   rpc.ClientConn
	client pb.ObjectDetectionServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewObjectDetectionServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections.
func (c *client) Close() error {
	return c.conn.Close()
}

// NewClient constructs a new client that is served at the given address.
func NewClient(ctx context.Context, name string, address string, logger golog.Logger, opts ...rpc.DialOption) (Service, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Service {
	return newSvcClientFromConn(conn, logger)
}

func (c *client) DetectorNames(ctx context.Context) ([]string, error) {
	resp, err := c.client.DetectorNames(ctx, &pb.DetectorNamesRequest{})
	if err != nil {
		return nil, err
	}
	return resp.DetectorNames, nil
}

func (c *client) AddDetector(ctx context.Context, cfg Config) (bool, error) {
	params, err := structpb.NewStruct(cfg.Parameters)
	if err != nil {
		return false, err
	}
	resp, err := c.client.AddDetector(ctx, &pb.AddDetectorRequest{
		DetectorName:       cfg.Name,
		DetectorModelType:  cfg.Type,
		DetectorParameters: params,
	})
	if err != nil {
		return false, err
	}
	return resp.Success, nil
}
