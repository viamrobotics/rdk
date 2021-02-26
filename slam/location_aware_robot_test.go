package slam

import (
	"context"
	"image"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/robots/fake"
	"go.viam.com/robotcore/testutils/inject"

	"github.com/edaniels/test"
)

type testHarness struct {
	bot        *LocationAwareRobot
	baseDevice *fake.Base
	area       *SquareArea
	lidarDev   *inject.LidarDevice
}

func newTestHarness(t *testing.T) *testHarness {
	return newTestHarnessWithLidar(t, nil)
}

func newTestHarnessWithLidar(t *testing.T, lidarDev lidar.Device) *testHarness {
	baseDevice := &fake.Base{}
	area := NewSquareArea(10, 10)
	baseStart := area.Center()
	injectLidarDev := &inject.LidarDevice{Device: lidarDev}
	if lidarDev == nil {
		injectLidarDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
			return image.Point{10, 10}, nil
		}
	}

	larBot, err := NewLocationAwareRobot(
		baseDevice,
		baseStart,
		area,
		[]lidar.Device{injectLidarDev},
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)
	return &testHarness{
		larBot,
		baseDevice,
		area,
		injectLidarDev,
	}
}
