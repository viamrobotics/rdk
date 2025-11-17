package video_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	pb "go.viam.com/api/service/video/v1"
	"go.viam.com/test"
	"go.viam.com/utils/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

// failingWriter is an io.Writer that always returns an error.
type failingWriter struct{ err error }

// Write always returns the predefined error.
func (fw *failingWriter) Write(p []byte) (int, error) {
	return 0, fw.err
}

var _ io.Writer = (*failingWriter)(nil)

func TestClientGetVideoStreamErrors(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectVideo := &inject.Video{}

	listener, cleanup := setupVideoService(t, injectVideo)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	svc, err := video.NewClientFromConn(ctx, conn, "", video.Named("video1"), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("extra conversion error (StructToStructPb fails)", func(t *testing.T) {
		extra := map[string]interface{}{"bad": make(chan int)}
		err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", "rid-1", extra, &bytes.Buffer{})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("context canceled before RPC", func(t *testing.T) {
		cctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		err := svc.GetVideo(cctx, time.Time{}, time.Time{}, "h264", "mp4", "rid-2", nil, &bytes.Buffer{})
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
	})

	t.Run("writer error during streaming", func(t *testing.T) {
		// Server writes one chunk, client writer fails.
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container, reqID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			_, err := w.Write([]byte{1, 2, 3})
			return err
		}
		w := &failingWriter{err: io.ErrClosedPipe}
		err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", "rid-4", nil, w)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, errors.Is(err, io.ErrClosedPipe), test.ShouldBeTrue)
	})

	t.Run("normal EOF with no data", func(t *testing.T) {
		// Server writes nothing and returns nil, client should return nil after io.EOF.
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container, reqID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			return nil
		}
		buf := &bytes.Buffer{}
		err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", "rid-5", nil, buf)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, buf.Len(), test.ShouldEqual, 0)
	})

	t.Run("context deadline exceeded while waiting for chunks", func(t *testing.T) {
		// Server blocks for a while, client times out.
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container, reqID string,
			extra map[string]interface{},
			w io.Writer,
		) error {
			// Simulate a long operation with no writes.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
				return nil
			}
		}
		shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		err := svc.GetVideo(shortCtx, time.Time{}, time.Time{}, "h264", "mp4", "rid-6", nil, &bytes.Buffer{})
		test.That(t, err, test.ShouldNotBeNil)
		code := status.Code(err)
		test.That(t, code == codes.DeadlineExceeded || code == codes.Canceled, test.ShouldBeTrue)
	})
}
