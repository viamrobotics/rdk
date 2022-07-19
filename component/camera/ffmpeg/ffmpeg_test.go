package ffmpeg

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

func TestFFmpegCamera(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()
	cam, err := NewFFmpegCamera(ctx, &ffmpegAttrs{URL: "rstp://10.1.1.29:8555/unicast"}, logger)
	test.That(t, err, test.ShouldBeNil)
	for i := 0; i < 10; i++ {
		img, _, err := cam.Next(ctx)
		logger.Debug("this shouldn't happen")
		test.That(t, err, test.ShouldBeNil)
		logger.Debug(img)
	}
}
