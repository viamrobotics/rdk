package lidar_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/core/lidar"
	"go.viam.com/core/rimage"
	"go.viam.com/core/testutils"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"
)

func TestImageSource(t *testing.T) {
	injectDev := &inject.Lidar{}
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

	outDir := testutils.TempDir(t, "", "lidar")
	golog.NewTestLogger(t).Debugf("out dir: %q", outDir)
	err = rimage.WriteImageToFile(outDir+"/out.png", img)
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
