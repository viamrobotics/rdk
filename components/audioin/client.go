package audioin

import (
	"context"
	"errors"
	"io"

	"github.com/google/uuid"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/audioin/v1"
	utils "go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

// client implements AudioInServiceClient.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.AudioInServiceClient
	logger logging.Logger
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (AudioIn, error) {
	serviceClient := pb.NewAudioInServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: serviceClient,
		logger: logger,
	}, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

func (c *client) GetAudio(ctx context.Context, codec string, durationSeconds float32, previousTimestampNs int64,
	extra map[string]interface{}) (chan *AudioChunk, error,
) {
	ext, err := utils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}

	// This only sets up the stream,it doesn't send the request to the server yet
	// The actual RPC call happens on first Recv()
	stream, err := c.client.GetAudio(ctx, &pb.GetAudioRequest{
		Name:                         c.name,
		DurationSeconds:              durationSeconds,
		Codec:                        codec,
		PreviousTimestampNanoseconds: previousTimestampNs,
		RequestId:                    uuid.New().String(),
		Extra:                        ext,
	})
	if err != nil {
		return nil, err
	}

	// receive one chunk outside of the goroutine to catch any errors
	resp, err := stream.Recv()
	if err != nil {
		return nil, err
	}

	// small buffered channel prevents blocking when receiver is temporarily slow
	ch := make(chan *AudioChunk, 8)

	go func() {
		defer close(ch)

		// Send the first response we already received
		var info *rutils.AudioInfo
		if resp.Audio.AudioInfo != nil {
			info = rutils.AudioInfoPBToStruct(resp.Audio.AudioInfo)
		}

		ch <- &AudioChunk{
			AudioData:                 resp.Audio.AudioData,
			AudioInfo:                 info,
			Sequence:                  resp.Audio.Sequence,
			StartTimestampNanoseconds: resp.Audio.StartTimestampNanoseconds,
			EndTimestampNanoseconds:   resp.Audio.EndTimestampNanoseconds,
			RequestID:                 resp.RequestId,
		}

		// Continue receiving the rest of the stream
		for {
			select {
			case <-ctx.Done():
				c.logger.Debugf("context done, returning from GetAudio: %v", ctx.Err())
				return
			default:
			}
			resp, err := stream.Recv()
			if err != nil {
				// EOF error indicates stream was closed by server.
				if !errors.Is(err, io.EOF) {
					c.logger.Error(err)
				}
				return
			}

			var info *rutils.AudioInfo
			if resp.Audio.AudioInfo != nil {
				info = rutils.AudioInfoPBToStruct(resp.Audio.AudioInfo)
			}

			ch <- &AudioChunk{
				AudioData:                 resp.Audio.AudioData,
				AudioInfo:                 info,
				Sequence:                  resp.Audio.Sequence,
				StartTimestampNanoseconds: resp.Audio.StartTimestampNanoseconds,
				EndTimestampNanoseconds:   resp.Audio.EndTimestampNanoseconds,
				RequestID:                 resp.RequestId,
			}
		}
	}()

	return ch, nil
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
		return rutils.Properties{}, err
	}

	return rutils.Properties{SupportedCodecs: resp.SupportedCodecs, SampleRateHz: resp.SampleRateHz, NumChannels: resp.NumChannels}, nil
}
