package ffmpeg

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils"
)

func TestFFmpegCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cam, err := NewFFmpegCamera(&ffmpegAttrs{URL: "rtsp://10.1.1.29:8555/unicast"}, logger)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 10; i++ {
		img, _, err := cam.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
		_ = img
	}
	test.That(t, utils.TryClose(context.Background(), cam), test.ShouldBeNil)
}
