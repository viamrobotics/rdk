package video_test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/test"
	"google.golang.org/grpc"
)

type testGetVideoServer struct {
	grpc.ServerStream
	ctx    context.Context
	writer io.Writer
}

func (x *testGetVideoServer) Context() context.Context {
	return x.ctx
}

func (x *testGetVideoServer) Send(m *pb.GetVideoResponse) error {
	_, err := x.writer.Write(m.VideoData)
	return err
}

func newServer() (pb.VideoServiceServer, *inject.Video, *inject.Video, error) {
	videoInject := &inject.Video{}
	videoInject2 := &inject.Video{}
	videos := map[resource.Name]video.Service{
		video.Named("video1"): videoInject,
		video.Named("video2"): videoInject2,
	}
	videoSvc, err := resource.NewAPIResourceCollection(video.API, videos)
	if err != nil {
		return nil, nil, nil, err
	}
	videoServer := video.NewRPCServiceServer(videoSvc).(pb.VideoServiceServer)
	return videoServer, videoInject, videoInject2, nil
}

func TestServer(t *testing.T) {
	videoServer, injectVideo, _, err := newServer()
	test.That(t, err, test.ShouldBeNil)

	getVideoRequest := &pb.GetVideoRequest{
		Name:           "video1",
		VideoCodec:     "h264",
		VideoContainer: "mp4",
		RequestId:      "12345",
	}

	t.Run("GetVideo success", func(t *testing.T) {
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			startTime, endTime time.Time,
			videoCodec, videoContainer, requestID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			data := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
			_, err := w.Write(data)
			return err
		}
		stream := &testGetVideoServer{ctx: context.Background()}
		err := videoServer.GetVideo(getVideoRequest, stream)
		test.That(t, err, test.ShouldBeNil)
		// Verify that the video data was sent
		expectedData := []byte{0x00, 0x01, 0x02, 0x03, 0x04}
		test.That(t, stream.writer.(*bytes.Buffer).Bytes(), test.ShouldResemble, expectedData)
	})

	t.Run("GetVideo failure", func(t *testing.T) {
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			startTime, endTime time.Time,
			videoCodec, videoContainer, requestID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			return io.EOF
		}
		stream := &testGetVideoServer{ctx: context.Background(), writer: &bytes.Buffer{}}
		err := videoServer.GetVideo(getVideoRequest, stream)
		test.That(t, err, test.ShouldNotBeNil)
	})
}
