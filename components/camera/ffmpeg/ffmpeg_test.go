package ffmpeg

import (
	"context"
	"os"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/logging"
)

func TestFFMPEGCamera(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	path := artifact.MustPath("components/camera/ffmpeg/testsrc.mpg")
	cam, err := NewFFMPEGCamera(ctx, &Config{VideoPath: path}, logger)
	test.That(t, err, test.ShouldBeNil)
	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 5; i++ {
		_, _, err := stream.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
	}
	test.That(t, stream.Close(context.Background()), test.ShouldBeNil)
	test.That(t, cam.Close(context.Background()), test.ShouldBeNil)
}

func TestFFMPEGNotFound(t *testing.T) {
	oldpath := os.Getenv("PATH")
	defer func() {
		os.Setenv("PATH", oldpath)
	}()
	os.Unsetenv("PATH")
	_, err := NewFFMPEGCamera(context.Background(), nil, nil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
}
