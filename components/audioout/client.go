package audioout

import (
	"context"
	"errors"
	"fmt"
	"io"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioout/v1"
	utils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// client implements AudioOutServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.AudioOutServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (AudioOut, error) {
	c := pb.NewAudioOutServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) Status(ctx context.Context) (map[string]interface{}, error) {
	return protoutils.GetStatusFromResourceClient(ctx, c.client, c.name)
}

func (c *client) Properties(ctx context.Context, extra map[string]interface{}) (rutils.Properties, error) {
	ext, err := utils.StructToStructPb(extra)
	if err != nil {
		return rutils.Properties{}, err
	}
	resp, err := c.client.GetProperties(ctx, &commonpb.GetPropertiesRequest{
		Name:  c.name,
		Extra: ext,
	})
	if err != nil {
		return rutils.Properties{}, fmt.Errorf("audioout client: could not get properties %w", err)
	}

	return rutils.Properties{SupportedCodecs: resp.SupportedCodecs, SampleRateHz: resp.SampleRateHz, NumChannels: resp.NumChannels}, nil
}

func (c *client) Play(ctx context.Context, data []byte, info *rutils.AudioInfo, extra map[string]interface{}) error {
	ext, err := utils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	req := &pb.PlayRequest{
		Name:      c.name,
		AudioData: data,
		Extra:     ext,
	}

	if info != nil {
		pbInfo := rutils.AudioInfoStructToPb(info)
		req.AudioInfo = pbInfo
	}

	_, err = c.client.Play(ctx, req)
	if err != nil {
		return fmt.Errorf("audioout client: could not play audio: %w", err)
	}
	return nil
}

func (c *client) PlayStream(ctx context.Context, info *rutils.AudioInfo, chunks <-chan []byte, extra map[string]interface{}) error {
	ext, err := utils.StructToStructPb(extra)
	if err != nil {
		return err
	}

	stream, err := c.client.PlayStream(ctx)
	if err != nil {
		return fmt.Errorf("audioout client: PlayStream: %w", err)
	}

	init := &pb.PlayStreamRequest{
		Payload: &pb.PlayStreamRequest_Init{
			Init: &pb.PlayStreamInit{Name: c.name, Extra: ext},
		},
	}
	if info != nil {
		init.GetInit().AudioInfo = rutils.AudioInfoStructToPb(info)
	}
	if err := stream.Send(init); err != nil {
		// On io.EOF the server closed the stream early; the reason is in CloseAndRecv.
		if errors.Is(err, io.EOF) {
			if _, recvErr := stream.CloseAndRecv(); recvErr != nil {
				return fmt.Errorf("audioout client: send init: %w", recvErr)
			}
			return fmt.Errorf("audioout client: send init: stream closed unexpectedly")
		}
		return fmt.Errorf("audioout client: send init: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-chunks:
			if !ok {
				_, err := stream.CloseAndRecv()
				if err != nil {
					return fmt.Errorf("audioout client: close and recv: %w", err)
				}
				return nil
			}
			msg := &pb.PlayStreamRequest{
				Payload: &pb.PlayStreamRequest_AudioChunk{
					AudioChunk: &pb.PlayStreamChunk{AudioData: chunk},
				},
			}
			if err := stream.Send(msg); err != nil {
				// On io.EOF the server closed the stream early; the reason is in CloseAndRecv.
				if errors.Is(err, io.EOF) {
					if _, recvErr := stream.CloseAndRecv(); recvErr != nil {
						return fmt.Errorf("audioout client: send chunk: %w", recvErr)
					}
					return fmt.Errorf("audioout client: send chunk: stream closed unexpectedly")
				}
				return fmt.Errorf("audioout client: send chunk: %w", err)
			}
		}
	}
}
