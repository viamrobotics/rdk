package rtsp

import (
	"context"
	"testing"
	"time"

	"github.com/bhaney/rtsp-simple-server/server"
	"github.com/edaniels/golog"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

func TestRTSPCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	// set up the rtsp simple server running on rtsp://localhost:8598/mystream
	outputURL := "rtsp://127.0.0.1:8598/mystream"
	configLoc := []string{artifact.MustPath("components/camera/rtsp/rtsp-simple-server.yml")}
	logger.Info("before server New")
	s, ok := server.New(configLoc)
	logger.Info("after server New")
	test.That(t, ok, test.ShouldBeTrue)
	defer s.Close()
	// run the test mp4 in a loop through ffmpeg to hand it to the server
	cancelCtx, cancel := context.WithCancel(context.Background())
	logger.Info("before ffmpeg run")
	viamutils.PanicCapturingGo(func() {
		testMP4 := artifact.MustPath("components/camera/rtsp/earth_480_video.mp4")
		ffmpegStream := ffmpeg.Input(testMP4, ffmpeg.KwArgs{"readrate": 1, "stream_loop": -1})
		ffmpegStream = ffmpegStream.Output(outputURL, ffmpeg.KwArgs{"c:v": "libx264", "f": "rtsp", "rtsp_transport": "tcp"})
		ffmpegStream.Context = cancelCtx
		cmd := ffmpegStream.OverWriteOutput().Compile()
		if err := cmd.Run(); err != nil {
			if viamutils.FilterOutError(err, context.Canceled) == nil ||
				viamutils.FilterOutError(err, context.DeadlineExceeded) == nil {
				return
			}
			if cmd.ProcessState.ExitCode() != 0 {
				panic(err)
			}
		}
	})
	logger.Info("after ffmpeg run")
	time.Sleep(5 * time.Second)
	logger.Info("after some 5 second")
	// create the rtsp camera model
	rtspConf := &Attrs{Address: outputURL}
	rtspCam, err := NewRTSPCamera(context.Background(), rtspConf, logger)
	test.That(t, err, test.ShouldBeNil)
	// get some frames from the image
	stream, err := rtspCam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	logger.Info("about to request 5 images")
	for i := 0; i < 5; i++ {
		_, _, err := stream.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
	}
	logger.Info("done requesting 5 images")
	// close everything
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
	logger.Info("about to close camera ")
	err = viamutils.TryClose(context.Background(), rtspCam)
	test.That(t, err, test.ShouldBeNil)
	logger.Info("closed camera ")
	logger.Info("about to cancel ffmpeg ")
	cancel()
	logger.Info("canceled ffmpeg")
}
