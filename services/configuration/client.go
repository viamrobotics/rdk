package configuration

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pion/mediadevices/pkg/driver"
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/prop"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/grpc"
	pb "go.viam.com/rdk/proto/api/service/configuration/v1"
)

type client struct {
	conn   rpc.ClientConn
	client pb.ConfigurationServiceClient
	logger golog.Logger
}

func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *client {
	grpcClient := pb.NewConfigurationServiceClient(conn)
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

func (c *client) GetCameras(ctx context.Context) ([]CameraConfig, error) {
	resp, err := c.client.GetCameras(ctx, &pb.GetCamerasRequest{})
	if err != nil {
		return nil, err
	}
	result := []CameraConfig{}
	for _, conf := range resp.GetCameras() {
		camConf := CameraConfig{
			Label:      conf.Label,
			Status:     driver.State(conf.Status),
			Properties: []prop.Media{},
		}

		for _, p := range conf.Properties {
			property := prop.Media{
				Video: prop.Video{
					Width:       int(p.Video.Width),
					Height:      int(p.Video.Height),
					FrameFormat: frame.Format(p.Video.FrameFormat),
				},
			}
			camConf.Properties = append(camConf.Properties, property)
		}
		result = append(result, camConf)
	}
	return result, nil
}
