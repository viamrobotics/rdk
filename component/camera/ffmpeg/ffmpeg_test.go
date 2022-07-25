package ffmpeg

import (
	"context"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	viamutils "go.viam.com/utils"
	"go.viam.com/utils/artifact"
)

func TestFFMPEGCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	path := artifact.MustPath("component/camera/ffmpeg/testsrc.mpg")
	cam, err := NewFFMPEGCamera(&AttrConfig{VideoPath: path}, logger)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 5; i++ {
		_, _, err := cam.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
	}
	test.That(t, viamutils.TryClose(context.Background(), cam), test.ShouldBeNil)
}

func TestFFMPEGNotFound(t *testing.T) {
	oldpath := os.Getenv("PATH")
	defer func() {
		os.Setenv("PATH", oldpath)
	}()
	os.Unsetenv("PATH")
	_, err := NewFFMPEGCamera(nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
}
