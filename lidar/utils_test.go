package lidar_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/testutils/inject"

	"go.viam.com/test"
)

func TestBestAngularResolution(t *testing.T) {
	lidar1 := &inject.LidarDevice{}
	lidar1.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}
	lidar2 := &inject.LidarDevice{}
	lidar2.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return math.NaN(), errors.New("whoops")
	}

	_, _, _, err := lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	lidar1.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return 1, nil
	}
	_, _, _, err = lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "whoops")

	lidar2.AngularResolutionFunc = func(ctx context.Context) (float64, error) {
		return .25, nil
	}
	best, bestDevice, bestDeviceNum, err := lidar.BestAngularResolution(context.Background(), []lidar.Device{lidar1, lidar2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, best, test.ShouldEqual, .25)
	test.That(t, bestDevice, test.ShouldEqual, lidar2)
	test.That(t, bestDeviceNum, test.ShouldEqual, 1)
}
