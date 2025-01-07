package discovery

import (
	"context"

	"go.opencensus.io/trace"
	pb "go.viam.com/api/service/discovery/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements DiscoveryServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.DiscoveryServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from the connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	grpcClient := pb.NewDiscoveryServiceClient(conn)
	c := &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: grpcClient,
		logger: logger,
	}
	return c, nil
}

func (c *client) DiscoverResources(ctx context.Context, extra map[string]any) ([]resource.Config, error) {
	ctx, span := trace.StartSpan(ctx, "discovery::client::DoCommand")
	defer span.End()
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	req := &pb.DiscoverResourcesRequest{Name: c.name, Extra: ext}
	resp, err := c.client.DiscoverResources(ctx, req)
	if err != nil {
		return nil, err
	}
	protoConfigs := resp.GetDiscoveries()
	if protoConfigs == nil {
		return nil, ErrNilResponse
	}

	discoveredConfigs := []resource.Config{}
	for _, proto := range protoConfigs {
		config, err := config.ComponentConfigFromProto(proto)
		if err != nil {
			return nil, err
		}
		discoveredConfigs = append(discoveredConfigs, *config)
	}
	return discoveredConfigs, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "discovery::client::DoCommand")
	defer span.End()

	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
