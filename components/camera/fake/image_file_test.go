package fake

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/utils"
)

func TestPCD(t *testing.T) {
	pcdPath := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
	cfg := &fileSourceConfig{PointCloud: pcdPath}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// TODO(hexbabe): remove below test when Stream/ReadImage pattern is refactored
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

	readInImage, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// TODO(hexbabe): remove below test when Stream/ReadImage pattern is refactored
	t.Run("test Stream and Next", func(t *testing.T) {
		stream, err := cam.Stream(ctx)
		test.That(t, err, test.ShouldBeNil)
		strmImg, _, err := stream.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, strmImg, test.ShouldResemble, readInImage)
		test.That(t, strmImg.Bounds(), test.ShouldResemble, readInImage.Bounds())
	})

	imgBytes, _, err := cam.Image(ctx, utils.MimeTypeJPEG, nil)
	test.That(t, err, test.ShouldBeNil)
	expectedBytes, err := rimage.EncodeImage(ctx, readInImage, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgBytes, test.ShouldResemble, expectedBytes)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestColor(t *testing.T) {
	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg := &fileSourceConfig{Color: colorImgPath}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = cam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldNotBeNil)

	readInImage, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	// TODO(hexbabe): remove below test when Stream/ReadImage pattern is refactored
	t.Run("test Stream and Next", func(t *testing.T) {
		stream, err := cam.Stream(ctx)
		test.That(t, err, test.ShouldBeNil)
		strmImg, _, err := stream.Next(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, strmImg, test.ShouldResemble, readInImage)
		test.That(t, strmImg.Bounds(), test.ShouldResemble, readInImage.Bounds())
	})

	imgBytes, _, err := cam.Image(ctx, utils.MimeTypeJPEG, nil)
	test.That(t, err, test.ShouldBeNil)
	expectedBytes, err := rimage.EncodeImage(ctx, readInImage, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgBytes, test.ShouldResemble, expectedBytes)

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
	logger := logging.NewTestLogger(t)
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// TODO(hexbabe): remove below test when Stream/ReadImage pattern is refactored
	t.Run("test Stream and Next", func(t *testing.T) {
		stream, err := cam.Stream(ctx)
		test.That(t, err, test.ShouldBeNil)

		readInImage, err := rimage.NewImageFromFile(imgFilePath)
		test.That(t, err, test.ShouldBeNil)

		strmImg, _, err := stream.Next(ctx)
		test.That(t, err, test.ShouldBeNil)

		expectedBounds := image.Rect(0, 0, readInImage.Bounds().Dx()-1, readInImage.Bounds().Dy()-1)
		test.That(t, strmImg, test.ShouldResemble, readInImage.SubImage(expectedBounds))
	})

	strmImg, err := camera.DecodeImageFromCamera(ctx, utils.MimeTypeRawRGBA, nil, cam)
	test.That(t, err, test.ShouldBeNil)

	expectedBounds := image.Rect(0, 0, img.Bounds().Dx()-1, img.Bounds().Dy()-1)
	test.That(t, strmImg.Bounds(), test.ShouldResemble, expectedBounds)
	val, _, err := rimage.CompareImages(strmImg, img.SubImage(expectedBounds))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, val, test.ShouldEqual, 0)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
