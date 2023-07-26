package powersensor

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/powersensor/v1"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements PowerSensorServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.PowerSensorServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger golog.Logger,
) (PowerSensor, error) {
	c := pb.NewPowerSensorServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		client: c,
		logger: logger,
	}, nil
}

func (c *client) Voltage(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, false, err
	}
	resp, err := c.client.GetVoltage(ctx, &pb.GetVoltageRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, false, err
	}
	return resp.Volts,
		resp.IsAc,
		nil
}

func (c *client) Current(ctx context.Context, extra map[string]interface{}) (float64, bool, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, false, err
	}
	resp, err := c.client.GetCurrent(ctx, &pb.GetCurrentRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, false, err
	}
	return resp.Amperes,
		resp.IsAc,
		nil
}

func (c *client) Power(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ext, err := structpb.NewStruct(extra)
	if err != nil {
		return 0, err
	}
	resp, err := c.client.GetPower(ctx, &pb.GetPowerRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return 0, err
	}
	return resp.Watts, nil
}

func (c *client) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return Readings(ctx, c, extra)
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
