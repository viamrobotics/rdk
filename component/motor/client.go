// Package motor contains a gRPC bases motor client
package motor

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/generic"
	pb "go.viam.com/rdk/proto/api/component/motor/v1"
	"go.viam.com/rdk/protoutils"
)

// serviceClient is a client that satisfies the motor.proto contract.
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.MotorServiceClient
	logger golog.Logger
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewMotorServiceClient(conn)
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// client is a motor client.
type client struct {
	*serviceClient
	name string
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Motor {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Motor {
	return &client{sc, name}
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

func (c *client) GoFor(ctx context.Context, rpm float64, revolutions float64, extra map[string]interface{}) error {
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

func (c *client) GoTo(ctx context.Context, rpm float64, positionRevolutions float64, extra map[string]interface{}) error {
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

func (c *client) GetPosition(ctx context.Context, extra map[string]interface{}) (float64, error) {
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

func (c *client) GetFeatures(ctx context.Context, extra map[string]interface{}) (map[Feature]bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	req := &pb.GetFeaturesRequest{Name: c.name, Extra: ext}
	resp, err := c.client.GetFeatures(ctx, req)
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

func (c *client) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return false, err
	}
	req := &pb.IsPoweredRequest{Name: c.name, Extra: ext}
	resp, err := c.client.IsPowered(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetIsOn(), nil
}

func (c *client) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return generic.DoFromConnection(ctx, c.conn, c.name, cmd)
}
