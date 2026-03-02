// Package ptz contains a gRPC based PTZ client.
package ptz

import (
	"context"
	"errors"

	pb "go.viam.com/api/component/ptz/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client implements PTZServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.PTZServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (PTZ, error) {
	c := pb.NewPTZServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) GetStatus(ctx context.Context, extra map[string]interface{}) (*Status, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetStatus(ctx, &pb.GetStatusRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return &Status{
		Position:      resp.Position,
		PanTiltStatus: resp.PanTiltStatus,
		ZoomStatus:    resp.ZoomStatus,
		UtcTime:       resp.UtcTime,
	}, nil
}

func (c *client) GetCapabilities(ctx context.Context, extra map[string]interface{}) (*Capabilities, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.GetCapabilities(ctx, &pb.GetCapabilitiesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return nil, err
	}
	return &Capabilities{
		MoveCapabilities: resp.MoveCapabilities,
		SupportsStatus:   resp.SupportsStatus,
		SupportsStop:     resp.SupportsStop,
	}, nil
}

func (c *client) Stop(ctx context.Context, panTilt, zoom *bool, extra map[string]interface{}) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	_, err = c.client.Stop(ctx, &pb.StopRequest{
		Name:    c.name,
		PanTilt: panTilt,
		Zoom:    zoom,
		Extra:   ext,
	})
	return err
}

func (c *client) Move(ctx context.Context, cmd *MoveCommand, extra map[string]interface{}) error {
	if cmd == nil {
		return errors.New("move command is required")
	}
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
	}
	req := &pb.MoveRequest{
		Name:  c.name,
		Extra: ext,
	}
	switch {
	case cmd.Continuous != nil:
		req.Command = &pb.MoveRequest_Continuous{Continuous: cmd.Continuous}
	case cmd.Relative != nil:
		req.Command = &pb.MoveRequest_Relative{Relative: cmd.Relative}
	case cmd.Absolute != nil:
		req.Command = &pb.MoveRequest_Absolute{Absolute: cmd.Absolute}
	default:
		return errors.New("move command must include a continuous, relative, or absolute request")
	}
	_, err = c.client.Move(ctx, req)
	return err
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
