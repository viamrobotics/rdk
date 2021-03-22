package lidar_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"
)

func TestImageSource(t *testing.T) {
	injectDev := &inject.LidarDevice{}
	err1 := errors.New("whoops1")
	err2 := errors.New("whoops2")
	injectDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{}, err1
	}
	injectDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, err2
	}

	src := lidar.NewImageSource(injectDev)
	_, _, err := src.Next(context.Background())
	test.That(t, err, test.ShouldEqual, err1)

	injectDev.BoundsFunc = func(ctx context.Context) (image.Point, error) {
		return image.Point{1, 1}, nil
	}

	_, _, err = src.Next(context.Background())
	test.That(t, err, test.ShouldEqual, err2)

	ms := lidar.Measurements{
		lidar.NewMeasurement(0, 0),
		lidar.NewMeasurement(1, 0.1),
		lidar.NewMeasurement(45, 0.2),
		lidar.NewMeasurement(220, 0.5),
		lidar.NewMeasurement(350, 0.4),
	}
	injectDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return ms, nil
	}

	img, release, err := src.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()

	points := utils.NewStringSet(
		"17,11",
		"43,89",
		"50,50",
		"50,59",
		"64,64",
	)
	count := 0
	rimage.IterateImage(img, func(x, y int, c color.Color) bool {
		rC := rimage.NewColorFromColor(c)
		if rC == rimage.Red {
			count++
			delete(points, fmt.Sprintf("%d,%d", x, y))
		}
		return true
	})
	test.That(t, count, test.ShouldEqual, 5)
	test.That(t, points, test.ShouldBeEmpty)
}
