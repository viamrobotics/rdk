package rtsp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/bhaney/rtsp-simple-server/server"
	"github.com/edaniels/golog"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
)

func startFFMPEG(outputURL string) context.CancelFunc {
	// run the test avi in a loop through ffmpeg and hand it to the server
	cancelCtx, cancel := context.WithCancel(context.Background())
	viamutils.PanicCapturingGo(func() {
		testMP4 := artifact.MustPath("components/camera/rtsp/earth_480_mjpeg.avi")
		ffmpegStream := ffmpeg.Input(testMP4, ffmpeg.KwArgs{"re": "", "stream_loop": -1})
		ffmpegStream = ffmpegStream.Output(outputURL, ffmpeg.KwArgs{"vcodec": "mjpeg", "huffman": "0", "f": "rtsp", "rtsp_transport": "tcp"})
		ffmpegStream.Context = cancelCtx
		cmd := ffmpegStream.OverWriteOutput().Compile()
		if err := cmd.Run(); err != nil {
			return
		}
	})
	return cancel
}

func startRTSPServer(t *testing.T) func() {
	t.Helper()
	configLoc := []string{artifact.MustPath("components/camera/rtsp/rtsp-simple-server.yml")}
	s, ok := server.New(configLoc)
	test.That(t, ok, test.ShouldBeTrue)
	return s.Close
}

func TestRTSPCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	host := "127.0.0.1"
	port := "32512"
	outputURL := fmt.Sprintf("rtsp://%s:%s/mystream", host, port)
	rtspConf := &Attrs{Address: outputURL}
	var rtspCam camera.Camera
	var err error
	// Just keep trying until you connect
	for {
		serverClose := startRTSPServer(t)
		cancel := startFFMPEG(outputURL)
		rtspCam, err = NewRTSPCamera(context.Background(), rtspConf, logger)
		// keep trying until the FFmpeg stream is running
		for err != nil && (strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "400")) {
			rtspCam, err = NewRTSPCamera(context.Background(), rtspConf, logger)
		}
		if err != nil && (strings.Contains(err.Error(), "timeout")) { // server is messed up, build it again
			cancel()
			serverClose()
			continue
		}
		defer cancel()
		defer serverClose()
		test.That(t, err, test.ShouldBeNil)
		break
	}
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
