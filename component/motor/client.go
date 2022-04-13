// Package motor contains a gRPC bases motor client
package motor

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/grpc"
	pbgeneric "go.viam.com/rdk/proto/api/component/generic/v1"
	pb "go.viam.com/rdk/proto/api/component/motor/v1"
)

// serviceClient is a client that satisfies the motor.proto contract.
type serviceClient struct {
	conn    rpc.ClientConn
	client  pb.MotorServiceClient
	gclient pbgeneric.GenericServiceClient
	logger  golog.Logger
}

// newServiceClient returns a new serviceClient served at the given address.
func newServiceClient(
	ctx context.Context,
	address string,
	logger golog.Logger,
	opts ...rpc.DialOption,
) (*serviceClient, error) {
	conn, err := grpc.Dial(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	sc := newSvcClientFromConn(conn, logger)
	return sc, nil
}

// newSvcClientFromConn constructs a new serviceClient using the passed in connection.
func newSvcClientFromConn(conn rpc.ClientConn, logger golog.Logger) *serviceClient {
	client := pb.NewMotorServiceClient(conn)
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

// client is a motor client.
type client struct {
	*serviceClient
	name string
}

// NewClient constructs a new client that is served at the given address.
func NewClient(
	ctx context.Context,
	name string,
	address string,
	logger golog.Logger,
	opts ...rpc.DialOption,
) (Motor, error) {
	sc, err := newServiceClient(ctx, address, logger, opts...)
	if err != nil {
		return nil, err
	}
	return clientFromSvcClient(sc, name), nil
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) Motor {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Motor {
	return &client{sc, name}
}

func (c *client) SetPower(ctx context.Context, powerPct float64) error {
	req := &pb.SetPowerRequest{
		Name:     c.name,
		PowerPct: powerPct,
	}
	_, err := c.client.SetPower(ctx, req)
	return err
}

func (c *client) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	req := &pb.GoForRequest{
		Name:        c.name,
		Rpm:         rpm,
		Revolutions: revolutions,
	}
	_, err := c.client.GoFor(ctx, req)
	return err
}

func (c *client) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	req := &pb.GoToRequest{
		Name:                c.name,
		Rpm:                 rpm,
		PositionRevolutions: positionRevolutions,
	}
	_, err := c.client.GoTo(ctx, req)
	return err
}

func (c *client) ResetZeroPosition(ctx context.Context, offset float64) error {
	req := &pb.ResetZeroPositionRequest{
		Name:   c.name,
		Offset: offset,
	}
	_, err := c.client.ResetZeroPosition(ctx, req)
	return err
}

func (c *client) GetPosition(ctx context.Context) (float64, error) {
	req := &pb.GetPositionRequest{Name: c.name}
	resp, err := c.client.GetPosition(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.GetPosition(), nil
}

func (c *client) GetFeatures(ctx context.Context) (map[Feature]bool, error) {
	req := &pb.GetFeaturesRequest{Name: c.name}
	resp, err := c.client.GetFeatures(ctx, req)
	if err != nil {
		return nil, err
	}
	return ProtoFeaturesToMap(resp), nil
}

func (c *client) Stop(ctx context.Context) error {
	req := &pb.StopRequest{Name: c.name}
	_, err := c.client.Stop(ctx, req)
	return err
}

func (c *client) IsPowered(ctx context.Context) (bool, error) {
	req := &pb.IsPoweredRequest{Name: c.name}
	resp, err := c.client.IsPowered(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetIsOn(), nil
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
