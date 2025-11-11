package video

import (
	"context"
	"errors"
	"io"
	"time"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/utils/protoutils"
	"go.viam.com/utils/rpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

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

func (c *client) GetVideo(
	ctx context.Context,
	startTime, endTime time.Time,
	videoCodec, videoContainer, requestID string,
	extra map[string]interface{},
	w io.Writer,
) error {
	ext, err := protoutils.StructToStructPb(extra)
	if err != nil {
		return err
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
		RequestId:      requestID,
		Extra:          ext,
	}
	stream, err := c.client.GetVideo(ctx, req)
	if err != nil {
		return err
	}
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		_, err = w.Write(chunk.GetVideoData())
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, err := protoutils.StructToStructPb(cmd)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.DoCommand(ctx, &commonpb.DoCommandRequest{
		Name:    c.name,
		Command: command,
	})
	if err != nil {
		return nil, err
	}
	return resp.Result.AsMap(), nil
}
