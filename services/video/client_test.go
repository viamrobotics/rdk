package video_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
	"go.viam.com/rdk/testutils/inject"
)

func setupVideoService(t *testing.T, injectVideo *inject.Video) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	videoSvc, err := resource.NewAPIResourceCollection(
		video.API, map[resource.Name]video.Service{video.Named("video1"): injectVideo})
	test.That(t, err, test.ShouldBeNil)
	resourceAPI, ok, err := resource.LookupAPIRegistration[video.Service](video.API)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, resourceAPI.RegisterRPCService(context.Background(), rpcServer, videoSvc), test.ShouldBeNil)

	go rpcServer.Serve(listener)
	return listener, func() { rpcServer.Stop() }
}

func TestFailingVideoClient(t *testing.T) {
	logger := logging.NewTestLogger(t)

	injectVideo := &inject.Video{}

	listener, cleanup := setupVideoService(t, injectVideo)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, context.Canceled)
}

func TestWorkingVideoClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectVideo := &inject.Video{}

	listener, cleanup := setupVideoService(t, injectVideo)
	defer cleanup()

	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := viamgrpc.Dial(cancelCtx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	videoClient := pb.NewVideoServiceClient(conn)

	t.Run("GetVideo success", func(t *testing.T) {
		// Mock video data to return
		mockVideoData := []byte{0x00, 0x01, 0x02, 0x03, 0x04}

		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			startTime time.Time,
			endTime time.Time,
			videoCodec string,
			videoContainer string,
			requestID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			_, err := w.Write(mockVideoData)
			return err
		}

		getVideoRequest := &pb.GetVideoRequest{
			Name:           "video1",
			VideoCodec:     "h264",
			VideoContainer: "mp4",
			RequestId:      "12345",
		}

		stream, err := videoClient.GetVideo(cancelCtx, getVideoRequest)
		test.That(t, err, test.ShouldBeNil)

		var receivedData []byte
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			test.That(t, err, test.ShouldBeNil)
			receivedData = append(receivedData, resp.VideoData...)
		}

		// Verify that the video data was received
		test.That(t, receivedData, test.ShouldResemble, mockVideoData)
	})

	t.Run("GetVideo failure", func(t *testing.T) {
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			startTime time.Time,
			endTime time.Time,
			videoCodec string,
			videoContainer string,
			requestID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			return io.EOF
		}

		getVideoRequest := &pb.GetVideoRequest{
			Name:           "video1",
			VideoCodec:     "h264",
			VideoContainer: "mp4",
			RequestId:      "12345",
		}

		stream, err := videoClient.GetVideo(cancelCtx, getVideoRequest)
		test.That(t, err, test.ShouldBeNil)

		_, err = stream.Recv()
		test.That(t, err, test.ShouldNotBeNil)
	})
}
