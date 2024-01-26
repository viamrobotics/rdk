package audioinput

import (
	"context"
	"io"
	"math"
	"sync"

	"github.com/pion/mediadevices/pkg/prop"
	"github.com/pion/mediadevices/pkg/wave"
	"github.com/pkg/errors"
	pb "go.viam.com/api/component/audioinput/v1"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is an audio input client.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	conn                    rpc.ClientConn
	client                  pb.AudioInputServiceClient
	logger                  logging.Logger
	mu                      sync.Mutex
	name                    string
	activeBackgroundWorkers sync.WaitGroup
	healthyClientCh         chan struct{}
}

// NewClientFromConn constructs a new Client from connection passed in.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (AudioInput, error) {
	c := pb.NewAudioInputServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.ShortName(),
		conn:   conn,
		client: c,
		logger: logger,
	}, nil
}

func (c *client) Read(ctx context.Context) (wave.Audio, func(), error) {
	stream, err := c.Stream(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err := stream.Close(ctx); err != nil {
			c.logger.CErrorw(ctx, "error closing stream", "error", err)
		}
	}()
	return stream.Next(ctx)
}

func (c *client) Stream(
	ctx context.Context,
	errHandlers ...gostream.ErrorHandler,
) (gostream.AudioStream, error) {
	// RSDK-6340: The resource manager closes remote resources when the underlying
	// connection goes bad. However, when the connection is re-established, the client
	// objects these resources represent are not re-initialized/marked "healthy".
	// `healthyClientCh` helps track these transitions between healthy and unhealthy
	// states.
	//
	// When a new `client.Stream()` is created we will either use the existing
	// `healthyClientCh` or create a new one.
	//
	// The goroutine a `Stream()` method spins off will listen to its version of the
	// `healthyClientCh` to be notified when the connection has died so it can gracefully
	// terminate.
	//
	// When a connection becomes unhealthy, the resource manager will call `Close` on the
	// audioinput client object. Closing the client will:
	// 1. close its `client.healthyClientCh` channel
	// 2. wait for existing "stream" goroutines to drain
	// 3. nil out the `client.healthyClientCh` member variable
	//
	// New streams concurrent with closing cannot start until this drain completes. There
	// will never be stream goroutines from the old "generation" running concurrently
	// with those from the new "generation".
	c.mu.Lock()
	if c.healthyClientCh == nil {
		c.healthyClientCh = make(chan struct{})
	}
	healthyClientCh := c.healthyClientCh
	c.mu.Unlock()

	streamCtx, stream, chunkCh := gostream.NewMediaStreamForChannel[wave.Audio](context.Background())

	chunksClient, err := c.client.Chunks(ctx, &pb.ChunksRequest{
		Name:         c.name,
		SampleFormat: pb.SampleFormat_SAMPLE_FORMAT_FLOAT32_INTERLEAVED,
	})
	if err != nil {
		return nil, err
	}

	infoResp, err := chunksClient.Recv()
	if err != nil {
		return nil, err
	}
	infoProto := infoResp.GetInfo()

	c.mu.Lock()
	if err := streamCtx.Err(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.activeBackgroundWorkers.Add(1)
	c.mu.Unlock()

	utils.PanicCapturingGo(func() {
		defer c.activeBackgroundWorkers.Done()
		defer close(chunkCh)

		for {
			if streamCtx.Err() != nil {
				return
			}

			var nextErr error

			chunkResp, err := chunksClient.Recv()

			var chunk wave.Audio
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				for _, handler := range errHandlers {
					handler(streamCtx, err)
				}
				nextErr = err
			} else {
				chunkProto := chunkResp.GetChunk()
				info := wave.ChunkInfo{
					Len:          int(chunkProto.Length),
					Channels:     int(infoProto.Channels),
					SamplingRate: int(infoProto.SamplingRate),
				}

				switch infoProto.SampleFormat {
				case pb.SampleFormat_SAMPLE_FORMAT_INT16_INTERLEAVED:
					chunkActual := wave.NewInt16Interleaved(info)
					for i := 0; i < info.Len; i++ {
						chunkActual.Data[i] = int16(HostEndian.Uint16(chunkProto.Data[i*2:]))
					}
					chunk = chunkActual
				case pb.SampleFormat_SAMPLE_FORMAT_FLOAT32_INTERLEAVED:
					chunkActual := wave.NewFloat32Interleaved(info)
					for i := 0; i < info.Len; i++ {
						chunkActual.Data[i] = math.Float32frombits(HostEndian.Uint32(chunkProto.Data[i*4:]))
					}
					chunk = chunkActual
				case pb.SampleFormat_SAMPLE_FORMAT_UNSPECIFIED:
					fallthrough
				default:
					nextErr = errors.Errorf("unknown type of audio sample format %v", infoProto.SampleFormat)
				}
			}

			select {
			case <-streamCtx.Done():
				return
			case <-healthyClientCh:
				if err := stream.Close(context.Background()); err != nil {
					c.logger.Warn("error closing stream", err)
				}
				return
			case chunkCh <- gostream.MediaReleasePairWithError[wave.Audio]{
				Media:   chunk,
				Release: func() {},
				Err:     nextErr,
			}:
			}
		}
	})

	return stream, nil
}

func (c *client) MediaProperties(ctx context.Context) (prop.Audio, error) {
	resp, err := c.client.Properties(ctx, &pb.PropertiesRequest{
		Name: c.name,
	})
	if err != nil {
		return prop.Audio{}, err
	}
	return prop.Audio{
		ChannelCount:  int(resp.ChannelCount),
		Latency:       resp.Latency.AsDuration(),
		SampleRate:    int(resp.SampleRate),
		SampleSize:    int(resp.SampleSize),
		IsBigEndian:   resp.IsBigEndian,
		IsFloat:       resp.IsFloat,
		IsInterleaved: resp.IsInterleaved,
	}, nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return protoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}

// TODO(RSDK-6433): This method can be called more than once during a client's lifecycle.
// For example, consider a case where a remote audioinput goes offline and then back
// online. We will call `Close` on the audioinput client when we detect the disconnection
// to remove active streams but then reuse the client when the connection is
// re-established.
func (c *client) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.healthyClientCh != nil {
		close(c.healthyClientCh)
	}
	c.activeBackgroundWorkers.Wait()
	c.healthyClientCh = nil
	return nil
}
