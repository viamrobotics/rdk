package audioout

import (
	"context"
	"fmt"

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
