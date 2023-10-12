package videosource

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
)

func TestPCD(t *testing.T) {
	pcdPath := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
	cfg := &fileSourceConfig{PointCloud: pcdPath}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	pc, err := cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	pc, err = cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg.Color = colorImgPath
	cam, err = newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	readInImage, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	strmImg, _, err := stream.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, strmImg, test.ShouldResemble, readInImage)
	test.That(t, strmImg.Bounds(), test.ShouldResemble, readInImage.Bounds())

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestColor(t *testing.T) {
	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg := &fileSourceConfig{Color: colorImgPath}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	readInImage, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	strmImg, _, err := stream.Next(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, strmImg, test.ShouldResemble, readInImage)
	test.That(t, strmImg.Bounds(), test.ShouldResemble, readInImage.Bounds())

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestColorOddResolution(t *testing.T) {
	imgFilePath := t.TempDir() + "/test_img.jpg"
	imgFile, err := os.Create(imgFilePath)
	test.That(t, err, test.ShouldBeNil)

	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for x := 0; x < img.Bounds().Dx(); x++ {
		for y := 0; y < img.Bounds().Dy(); y++ {
			img.Set(x, y, color.White)
		}
	}
	err = jpeg.Encode(imgFile, img, nil)
	test.That(t, err, test.ShouldBeNil)
	err = imgFile.Close()
	test.That(t, err, test.ShouldBeNil)

	cfg := &fileSourceConfig{Color: imgFilePath}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	stream, err := cam.Stream(ctx)
	test.That(t, err, test.ShouldBeNil)

	readInImage, err := rimage.NewImageFromFile(imgFilePath)
	test.That(t, err, test.ShouldBeNil)

	strmImg, _, err := stream.Next(ctx)
	test.That(t, err, test.ShouldBeNil)

	expectedBounds := image.Rect(0, 0, readInImage.Bounds().Dx()-1, readInImage.Bounds().Dy()-1)
	test.That(t, strmImg, test.ShouldResemble, readInImage.SubImage(expectedBounds))
	test.That(t, strmImg.Bounds(), test.ShouldResemble, expectedBounds)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
