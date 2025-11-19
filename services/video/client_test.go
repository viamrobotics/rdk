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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	viamgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/video"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testVideoName = "video1"
)

func setupVideoService(t *testing.T, injectVideo *inject.Video) (net.Listener, func()) {
	t.Helper()
	logger := logging.NewTestLogger(t)
	listener, err := net.Listen("tcp", "localhost:0")
	test.That(t, err, test.ShouldBeNil)
	rpcServer, err := rpc.NewServer(logger, rpc.WithUnauthenticated())
	test.That(t, err, test.ShouldBeNil)

	videoSvc, err := resource.NewAPIResourceCollection(
		video.API, map[resource.Name]video.Service{video.Named(testVideoName): injectVideo})
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
			extra map[string]interface{},
		) (chan *video.Chunk, error) {
			ch := make(chan *video.Chunk, 1)
			go func() {
				defer close(ch)
				ch <- &video.Chunk{
					Data:      mockVideoData,
					Container: "mp4",
				}
			}()
			return ch, nil
		}

		getVideoRequest := &pb.GetVideoRequest{
			Name:           testVideoName,
			VideoCodec:     "h264",
			VideoContainer: "mp4",
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
			extra map[string]interface{},
		) (chan *video.Chunk, error) {
			return nil, io.EOF
		}

		getVideoRequest := &pb.GetVideoRequest{
			Name:           testVideoName,
			VideoCodec:     "h264",
			VideoContainer: "mp4",
		}

		stream, err := videoClient.GetVideo(cancelCtx, getVideoRequest)
		test.That(t, err, test.ShouldBeNil)

		_, err = stream.Recv()
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestClientGetVideoStreamErrors(t *testing.T) {
	logger := logging.NewTestLogger(t)
	injectVideo := &inject.Video{}

	listener, cleanup := setupVideoService(t, injectVideo)
	defer cleanup()

	ctx := context.Background()
	conn, err := viamgrpc.Dial(ctx, listener.Addr().String(), logger)
	test.That(t, err, test.ShouldBeNil)
	defer conn.Close()

	svc, err := video.NewClientFromConn(ctx, conn, "", video.Named(testVideoName), logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("extra conversion error (StructToStructPb fails)", func(t *testing.T) {
		extra := map[string]interface{}{"bad": make(chan int)}
		ch, err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", extra)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, ch, test.ShouldBeNil)
	})

	t.Run("context canceled before RPC", func(t *testing.T) {
		cctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		ch, err := svc.GetVideo(cctx, time.Time{}, time.Time{}, "h264", "mp4", nil)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, status.Code(err), test.ShouldEqual, codes.Canceled)
		test.That(t, ch, test.ShouldBeNil)
	})

	t.Run("success one chunk via channel", func(t *testing.T) {
		// Server sends one chunk then closes.
		payload := []byte{9, 8, 7}
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container string,
			extra map[string]interface{},
		) (chan *video.Chunk, error) {
			ch := make(chan *video.Chunk, 1)
			go func() {
				defer close(ch)
				ch <- &video.Chunk{Data: payload, Container: container}
			}()
			return ch, nil
		}
		ch, err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", nil)
		test.That(t, err, test.ShouldBeNil)

		select {
		case got, ok := <-ch:
			test.That(t, ok, test.ShouldBeTrue)
			test.That(t, got.Data, test.ShouldResemble, payload)
			test.That(t, got.Container, test.ShouldEqual, "mp4")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for first chunk")
		}

		// Ensure channel closes after the first chunk.
		select {
		case _, ok := <-ch:
			test.That(t, ok, test.ShouldBeFalse)
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for channel close")
		}
	})

	t.Run("normal EOF with no data", func(t *testing.T) {
		// Server writes nothing and closes channel.
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container string,
			extra map[string]interface{},
		) (chan *video.Chunk, error) {
			ch := make(chan *video.Chunk)
			close(ch)
			return ch, nil
		}
		ch, err := svc.GetVideo(ctx, time.Time{}, time.Time{}, "h264", "mp4", nil)
		test.That(t, err, test.ShouldBeNil)
		select {
		case _, ok := <-ch:
			test.That(t, ok, test.ShouldBeFalse) // closed, no data
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for channel close")
		}
	})

	t.Run("context deadline exceeded while waiting for chunks", func(t *testing.T) {
		// Server blocks and client times out via context. The client GetVideo returns (ch, nil),
		// and the goroutine will close ch after Recv returns the context error.
		injectVideo.GetVideoFunc = func(
			ctx context.Context,
			s, e time.Time,
			codec, container string,
			extra map[string]interface{},
		) (chan *video.Chunk, error) {
			ch := make(chan *video.Chunk)
			// Simulate a producer that never sends (server blocks); channel remains open
			// and the server handler will terminate when ctx is done.
			go func() {
				<-ctx.Done()
				close(ch)
			}()
			return ch, nil
		}
		shortCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		ch, err := svc.GetVideo(shortCtx, time.Time{}, time.Time{}, "h264", "mp4", nil)
		test.That(t, err, test.ShouldBeNil)

		// Expect the channel to close without yielding any chunks within a short time.
		select {
		case _, ok := <-ch:
			test.That(t, ok, test.ShouldBeFalse)
		case <-time.After(1 * time.Second):
			t.Fatal("timed out waiting for channel close after context deadline")
		}
	})
}
