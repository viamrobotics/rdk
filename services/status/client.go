// Package status contains a gRPC based status service client
package status

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/status/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a client implements the StatusServiceClient.
type client struct {
	conn   rpc.ClientConn
	client pb.StatusServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewStatusServiceClient(conn)
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

func (c *client) GetStatus(ctx context.Context, resourceNames []resource.Name) ([]Status, error) {
	names := make([]*commonpb.ResourceName, 0, len(resourceNames))
	for _, name := range resourceNames {
		names = append(names, protoutils.ResourceNameToProto(name))
	}

	resp, err := c.client.GetStatus(ctx, &pb.GetStatusRequest{ResourceNames: names})
	if err != nil {
		return nil, err
	}

	statuses := make([]Status, 0, len(resp.Status))
	for _, status := range resp.Status {
		statuses = append(
			statuses, Status{
				Name:   protoutils.ResourceNameFromProto(status.Name),
				Status: status.Status.AsMap(),
			})
	}
	return statuses, nil
}
