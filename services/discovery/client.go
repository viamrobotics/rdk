package discovery

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/service/discovery/v1"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type client struct {
	conn   rpc.ClientConn
	client pb.DiscoveryServiceClient
	logger golog.Logger
}

func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewDiscoveryServiceClient(conn)
	sc := &client{
		conn:   conn,
		client: grpcClient,
		logger: logger,
	}
	return sc
}

func (c *client) Close(ctx context.Context) error {
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

func (c *client) Discover(ctx context.Context, subtypeNames []resource.SubtypeName) ([]Discovery, error) {
	names := make([]*commonpb.ResourceName, 0, len(subtypeNames))
	for _, name := range subtypeNames {
		names = append(names, protoutils.ResourceNameToProto(subtypeNames))
	}

	resp, err := c.client.Discover(ctx, &pb.DiscoverRequest{ResourceNames: names})
	if err != nil {
		return nil, err
	}

	discoveries := make([]Discovery, 0, len(resp.Discovery))
	for _, discovery := range resp.Discovery {
		discoveries = append(
			discoveries, Discovery{
				Name:       protoutils.ResourceNameFromProto(discovery.Name),
				Discovered: discovery.Discovered.AsMap(),
			})
	}
	return discoveries, nil
}
