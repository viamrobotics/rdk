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
	s, ok := server.New(configLoc)
	test.That(t, ok, test.ShouldBeTrue)
	defer s.Close()
	// run the test mp4 in a loop through ffmpeg to hand it to the server
	cancelCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	viamutils.PanicCapturingGo(func() {
		testMP4 := artifact.MustPath("components/camera/rtsp/earth_480_video.mp4")
		ffmpegStream := ffmpeg.Input(testMP4, ffmpeg.KwArgs{"re": "", "stream_loop": -1})
		ffmpegStream = ffmpegStream.Output(outputURL, ffmpeg.KwArgs{"c:v": "libx264", "f": "rtsp", "rtsp_transport": "tcp"})
		ffmpegStream.Context = cancelCtx
		cmd := ffmpegStream.OverWriteOutput().Compile()
		if err := cmd.Run(); err != nil {
			return
		}
	})
	time.Sleep(3 * time.Second) // wait so ffmpeg can publish to rtsp server
	// create the rtsp camera model
	rtspConf := &Attrs{Address: outputURL}
	rtspCam, err := NewRTSPCamera(context.Background(), rtspConf, logger)
	test.That(t, err, test.ShouldBeNil)
	// get some frames from the image
	stream, err := rtspCam.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, _, err = stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil) // request first picture
	for i := 0; i < 5; i++ {
		_, _, err := stream.Next(context.Background())
		test.That(t, err, test.ShouldBeNil)
	}
	// close everything
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
	err = viamutils.TryClose(context.Background(), rtspCam)
	test.That(t, err, test.ShouldBeNil)
}
