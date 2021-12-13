// Package motor contains a gRPC bases motor client
package motor

import (
	"context"
	"errors"

	"go.viam.com/utils/rpc"

	"github.com/edaniels/golog"

	"go.viam.com/core/grpc"
	pb "go.viam.com/core/proto/api/component/v1"
)

// serviceClient is a client that satisfies the motor.proto contract
type serviceClient struct {
	conn   rpc.ClientConn
	client pb.MotorServiceClient
	logger golog.Logger
}

// newServiceClient returns a new serviceClient served at the given address
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
	sc := &serviceClient{
		conn:   conn,
		client: client,
		logger: logger,
	}
	return sc
}

// Close cleanly closes the underlying connections
func (sc *serviceClient) Close() error {
	return sc.conn.Close()
}

// client is a motor client
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
func NewClientFromConn(conn rpc.ClientConn, name string, logger golog.Logger) Motor {
	sc := newSvcClientFromConn(conn, logger)
	return clientFromSvcClient(sc, name)
}

func clientFromSvcClient(sc *serviceClient, name string) Motor {
	return &client{sc, name}
}

func (c *client) SetPower(ctx context.Context, powerPct float64) error {
	req := &pb.MotorServiceSetPowerRequest{
		Name:     c.name,
		PowerPct: powerPct,
	}
	_, err := c.client.SetPower(ctx, req)
	return err
}

func (c *client) Go(ctx context.Context, powerPct float64) error {
	req := &pb.MotorServiceGoRequest{
		Name:     c.name,
		PowerPct: powerPct,
	}
	_, err := c.client.Go(ctx, req)
	return err
}

func (c *client) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	req := &pb.MotorServiceGoForRequest{
		Name:        c.name,
		Rpm:         rpm,
		Revolutions: revolutions,
	}
	_, err := c.client.GoFor(ctx, req)
	return err
}

func (c *client) GoTo(ctx context.Context, rpm float64, position float64) error {
	req := &pb.MotorServiceGoToRequest{
		Name:     c.name,
		Rpm:      rpm,
		Position: position,
	}
	_, err := c.client.GoTo(ctx, req)
	return err
}

func (c *client) GoTillStop(
	ctx context.Context,
	rpm float64,
	stopFunc func(ctx context.Context) bool,
) error {
	if stopFunc != nil {
		return errors.New("stopFunc must be nil when using gRPC")
	}
	req := &pb.MotorServiceGoTillStopRequest{
		Name: c.name,
		Rpm:  rpm,
	}
	_, err := c.client.GoTillStop(ctx, req)
	return err
}

func (c *client) ResetZeroPosition(ctx context.Context, offset float64) error {
	req := &pb.MotorServiceResetZeroPositionRequest{
		Name:   c.name,
		Offset: offset,
	}
	_, err := c.client.ResetZeroPosition(ctx, req)
	return err
}

func (c *client) Position(ctx context.Context) (float64, error) {
	req := &pb.MotorServicePositionRequest{Name: c.name}
	resp, err := c.client.Position(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.GetPosition(), nil
}

func (c *client) PositionSupported(ctx context.Context) (bool, error) {
	req := &pb.MotorServicePositionSupportedRequest{Name: c.name}
	resp, err := c.client.PositionSupported(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetSupported(), nil
}

func (c *client) Stop(ctx context.Context) error {
	req := &pb.MotorServiceStopRequest{Name: c.name}
	_, err := c.client.Stop(ctx, req)
	return err
}

func (c *client) IsOn(ctx context.Context) (bool, error) {
	req := &pb.MotorServiceIsOnRequest{Name: c.name}
	resp, err := c.client.IsOn(ctx, req)
	if err != nil {
		return false, err
	}
	return resp.GetIsOn(), nil
}

func (c *client) PID() PID {
	return nil
}
