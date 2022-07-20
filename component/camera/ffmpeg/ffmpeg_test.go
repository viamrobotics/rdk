package ffmpeg

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/utils"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
)

func TestFFmpegCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	path := utils.ResolveFile("component/camera/ffmpeg/data/testsrc.mpg")
	cam, err := NewFFmpegCamera(&ffmpegAttrs{Source: path}, logger)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 5; i++ {
		_, _, err := cam.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
	}
	test.That(t, viamutils.TryClose(context.Background(), cam), test.ShouldBeNil)
}

func TestComputerWithoutFFmpeg(t *testing.T) {
	oldpath := os.Getenv("PATH")
	defer func() {
		os.Setenv("PATH", oldpath)
	}()
	os.Unsetenv("PATH")
	_, err := NewFFmpegCamera(nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "\"ffmpeg\": executable file not found in $PATH")
}
