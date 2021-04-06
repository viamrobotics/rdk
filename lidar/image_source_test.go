package lidar_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/edaniels/test"
	"go.viam.com/robotcore/lidar"
	"go.viam.com/robotcore/rimage"
	"go.viam.com/robotcore/testutils/inject"
	"go.viam.com/robotcore/utils"
)

func TestImageSource(t *testing.T) {
	injectDev := &inject.LidarDevice{}
	err2 := errors.New("whoops2")
	injectDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return nil, err2
	}

	src := lidar.NewImageSource(image.Point{100, 100}, injectDev)

	_, _, err := src.Next(context.Background())
	test.That(t, err, test.ShouldEqual, err2)

	ms := lidar.Measurements{
		lidar.NewMeasurement(0, 0),   // 50,50
		lidar.NewMeasurement(0, 1),   // 50,?
		lidar.NewMeasurement(90, 2),  // ?,50
		lidar.NewMeasurement(180, 3), // 50,?
		lidar.NewMeasurement(270, 4), // ?,50
	}

	injectDev.ScanFunc = func(ctx context.Context, options lidar.ScanOptions) (lidar.Measurements, error) {
		return ms, nil
	}

	img, release, err := src.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	defer release()

	os.MkdirAll("out", 0775)
	err = rimage.WriteImageToFile("out/out.png", img)
	test.That(t, err, test.ShouldBeNil)

	delta := 8

	points := utils.NewStringSet(
		"50,50",
		fmt.Sprintf("50,%d", 50-delta),
		fmt.Sprintf("%d,50", 50+2*delta),
		fmt.Sprintf("50,%d", 50+3*delta),
		fmt.Sprintf("%d,50", 50-4*delta),
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
