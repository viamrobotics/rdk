package video

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// client is a video service client that talks to a gRPC video service.
type client struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name   string
	client pb.VideoServiceClient
	logger logging.Logger
}

// NewClientFromConn creates a new video service client from a gRPC connection.
func NewClientFromConn(
	ctx context.Context,
	conn rpc.ClientConn,
	remoteName string,
	name resource.Name,
	logger logging.Logger,
) (Service, error) {
	c := pb.NewVideoServiceClient(conn)
	return &client{
		Named:  name.PrependRemote(remoteName).AsNamed(),
		name:   name.Name,
		client: c,
		logger: logger,
	}, nil
}

// GetVideo calls the GetVideo method on the video service.
func (c *client) GetVideo(
	ctx context.Context,
	startTime, endTime time.Time,
	videoCodec, videoContainer string,
	extra map[string]interface{},
) (chan *Chunk, error) {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return nil, err
	}
	var startTS *timestamppb.Timestamp
	if !startTime.IsZero() {
		startTS = timestamppb.New(startTime)
	}
	var endTS *timestamppb.Timestamp
	if !endTime.IsZero() {
		endTS = timestamppb.New(endTime)
	}
	req := &pb.GetVideoRequest{
		Name:           c.name,
		StartTimestamp: startTS,
		EndTimestamp:   endTS,
		VideoCodec:     videoCodec,
		VideoContainer: videoContainer,
		RequestId:      uuid.New().String(),
		Extra:          ext,
	}
	stream, err := c.client.GetVideo(ctx, req)
	if err != nil {
		return nil, err
	}

	// small buffered channel to prevent blocking when receiver is slow
	ch := make(chan *Chunk, 8)
	go func() {
		defer close(ch)
		for {
			// The blocking Recv call below already honors ctx: if ctx is canceled or its deadline
			// expires, Recv returns an error (e.g. context.Canceled or context.DeadlineExceeded).
			// Therefore we do not need an explicit select on ctx.Done(). We only check for:
			//   - io.EOF: normal end of stream.
			//   - any other error: propagate upstream (includes context cancel).
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				c.logger.Errorf("error receiving video chunk: %v", err)
				return
			}
			ch <- &Chunk{
				Data:      chunk.GetVideoData(),
				Container: chunk.GetVideoContainer(),
				RequestID: chunk.GetRequestId(),
			}
		}
	}()

	return ch, nil
}

// DoCommand calls the DoCommand method on the video service.
func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "discovery::client::DoCommand")
	defer span.End()
	return rprotoutils.DoFromResourceClient(ctx, c.client, c.name, cmd)
}
