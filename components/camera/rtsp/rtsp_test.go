package rtsp

import (
	"context"
	"strings"
	"testing"

	"github.com/bhaney/rtsp-simple-server/server"
	"github.com/edaniels/golog"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

func startFFMPEG(outputURL string, streamStarted chan struct{}) context.CancelFunc {
	// run the test avi in a loop through ffmpeg and hand it to the server
	cancelCtx, cancel := context.WithCancel(context.Background())
	viamutils.PanicCapturingGo(func() {
		testMP4 := artifact.MustPath("components/camera/rtsp/earth_480_mjpeg.avi")
		ffmpegStream := ffmpeg.Input(testMP4, ffmpeg.KwArgs{"re": "", "stream_loop": -1})
		ffmpegStream = ffmpegStream.Output(outputURL, ffmpeg.KwArgs{"vcodec": "mjpeg", "huffman": "0", "f": "rtsp", "rtsp_transport": "tcp"})
		ffmpegStream.Context = cancelCtx
		cmd := ffmpegStream.OverWriteOutput().Compile()
		close(streamStarted)
		if err := cmd.Run(); err != nil {
			return
		}
	})
	return cancel
}

func TestRTSPCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// set up the rtsp simple server running on rtsp://localhost:8598/mystream
	outputURL := "rtsp://127.0.0.1:8598/mystream"
	configLoc := []string{artifact.MustPath("components/camera/rtsp/rtsp-simple-server.yml")}
	s, ok := server.New(configLoc)
	test.That(t, ok, test.ShouldBeTrue)
	defer s.Close()
	streamStarted := make(chan struct{})
	cancel := startFFMPEG(outputURL, streamStarted)
	defer cancel()
	// create the rtsp camera model
	rtspConf := &Attrs{Address: outputURL}
	<-streamStarted
	rtspCam, err := NewRTSPCamera(context.Background(), rtspConf, logger)
	// keep trying until RTSP server is running
	for err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "400")) {
		rtspCam, err = NewRTSPCamera(context.Background(), rtspConf, logger)
	}
	test.That(t, err, test.ShouldBeNil)
	stream, err := rtspCam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, _, err = stream.Next(context.Background()) // first Next to trigger the gotFirstFrame bool and chan
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 10; i++ {
		_, _, err := stream.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
	}
	// close everything
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
	err = viamutils.TryClose(context.Background(), rtspCam)
	test.That(t, err, test.ShouldBeNil)
}
