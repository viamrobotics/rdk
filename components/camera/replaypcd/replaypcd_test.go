package replaypcd

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestPCDCameraUnimplemented(t *testing.T) {
	logger := golog.NewTestLogger(t)
	ctx := context.Background()

	cfg := resource.Config{}

	replayCamera, err := newPCDCamera(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("NextPointCloud", func(t *testing.T) {
		_, err := replayCamera.NextPointCloud(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "NextPointCloud is unimplemented")
	})

	t.Run("Stream", func(t *testing.T) {
		_, err := replayCamera.Stream(ctx, nil)
		test.That(t, err.Error(), test.ShouldEqual, "Stream is unimplemented")
	})

	t.Run("Properties", func(t *testing.T) {
		_, err := replayCamera.Properties(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Properties is unimplemented")
	})

	t.Run("Projector", func(t *testing.T) {
		_, err := replayCamera.Projector(ctx)
		test.That(t, err.Error(), test.ShouldEqual, "Projector is unimplemented")
	})

	err = replayCamera.Close(ctx)
	test.That(t, err.Error(), test.ShouldEqual, "Close is unimplemented")
}
