// Package motor contains a gRPC bases motor client
package motor

import (
	"context"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/component/motor/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/components/generic"
)

// client implements MotorServiceClient.
type client struct {
	name   string
	conn   rpc.ClientConn
	client pb.MotorServiceClient
	logger golog.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Motor {
	c := pb.NewMotorServiceClient(conn)
	return &client{
		name:   name,
		conn:   conn,
		client: c,
		logger: logger,
	}
}

func (c *client) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.SetPowerRequest{
		Name:     c.name,
		PowerPct: powerPct,
		Extra:    ext,
	}
	_, err = c.client.SetPower(ctx, req)
	return err
}

func (c *client) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.GoForRequest{
		Name:        c.name,
		Rpm:         rpm,
		Revolutions: revolutions,
		Extra:       ext,
	}
	_, err = c.client.GoFor(ctx, req)
	return err
}

func (c *client) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.GoToRequest{
		Name:                c.name,
		Rpm:                 rpm,
		PositionRevolutions: positionRevolutions,
		Extra:               ext,
	}
	_, err = c.client.GoTo(ctx, req)
	return err
}

func (c *client) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.ResetZeroPositionRequest{
		Name:   c.name,
		Offset: offset,
		Extra:  ext,
	}
	_, err = c.client.ResetZeroPosition(ctx, req)
	return err
}

func (c *client) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return 0, err
	}
	req := &pb.GetPositionRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.GetPosition(), nil
}

func (c *client) Properties(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	req := &pb.GetPropertiesRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetProperties(ctx, req)
	if err != nil {
		return nil, err
	}
	return ProtoFeaturesToMap(resp), nil
}

func (c *client) Stop(ctx context.Context, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.StopRequest{Name: c.name, Extra: ext}
	_, err = c.client.Stop(ctx, req)
	return err
}

func (c *client) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, 0.0, err
	}
	req := &pb.IsPoweredRequest{Name: c.name, Extra: ext}
	resp, err := c.client.IsPowered(ctx, req)
	if err != nil {
		return false, 0.0, err
	}
	return resp.GetIsOn(), resp.GetPowerPct(), nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}

func (c *client) IsMoving(ctx context.Context) (bool, error) {
	resp, err := c.client.IsMoving(ctx, &pb.IsMovingRequest{Name: c.name})
	if err != nil {
		return false, err
	}
	return resp.IsMoving, nil
}
