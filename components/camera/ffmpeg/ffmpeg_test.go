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
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 5; i++ {
		namedImages, _, err := cam.Images(ctx, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(namedImages) > 0, test.ShouldBeTrue)
		_, err = namedImages[0].Image(ctx)
		test.That(t, err, test.ShouldBeNil)
	}
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
