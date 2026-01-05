package fake

import (
	"context"
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

	pc, err := cam.NextPointCloud(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	pc, err = cam.NextPointCloud(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 628)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)

	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg.Color = colorImgPath
	cam, err = newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	readInImage, err := rimage.ReadImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	namedImages, _, err := cam.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	imgBytes, err := namedImages[0].Bytes(ctx)
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

	_, err = cam.NextPointCloud(ctx, nil)
	test.That(t, err, test.ShouldNotBeNil)

	readInImage, err := rimage.ReadImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)

	namedImages, _, err := cam.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	imgBytes, err := namedImages[0].Bytes(ctx)
	test.That(t, err, test.ShouldBeNil)
	expectedBytes, err := rimage.EncodeImage(ctx, readInImage, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgBytes, test.ShouldResemble, expectedBytes)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}

func TestPreloadedImages(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	preloadedImages := []string{"pizza", "dog", "crowd"}

	for _, imgName := range preloadedImages {
		t.Run(imgName, func(t *testing.T) {
			cfg := &fileSourceConfig{PreloadedImage: imgName}
			cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
			test.That(t, err, test.ShouldBeNil)

			namedImages, metadata, err := cam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(namedImages), test.ShouldEqual, 1)
			test.That(t, namedImages[0].SourceName, test.ShouldEqual, "preloaded")
			test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)

			img, err := namedImages[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, img, test.ShouldNotBeNil)

			bounds := img.Bounds()
			test.That(t, bounds.Dx() > 0, test.ShouldBeTrue)
			test.That(t, bounds.Dy() > 0, test.ShouldBeTrue)

			jpegBytes, err := namedImages[0].Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, namedImages[0].MimeType(), test.ShouldEqual, utils.MimeTypeJPEG)
			test.That(t, len(jpegBytes) > 0, test.ShouldBeTrue)

			err = cam.Close(ctx)
			test.That(t, err, test.ShouldBeNil)
		})
	}

	colorImgPath := artifact.MustPath("vision/objectdetection/detection_test.jpg")
	cfg := &fileSourceConfig{
		PreloadedImage: "pizza",
		Color:          colorImgPath,
	}
	cam, err := newCamera(ctx, resource.Name{API: camera.API}, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// Should return both images
	namedImages, _, err := cam.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(namedImages), test.ShouldEqual, 2)
	test.That(t, namedImages[0].SourceName, test.ShouldEqual, "preloaded")
	test.That(t, namedImages[1].SourceName, test.ShouldEqual, "color")

	// Should return only preloaded
	namedImages, _, err = cam.Images(ctx, []string{"preloaded"}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(namedImages), test.ShouldEqual, 1)
	test.That(t, namedImages[0].SourceName, test.ShouldEqual, "preloaded")

	// Should return only color
	namedImages, _, err = cam.Images(ctx, []string{"color"}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(namedImages), test.ShouldEqual, 1)
	test.That(t, namedImages[0].SourceName, test.ShouldEqual, "color")

	// Should return both
	namedImages, _, err = cam.Images(ctx, []string{"preloaded", "color"}, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(namedImages), test.ShouldEqual, 2)
	test.That(t, namedImages[0].SourceName, test.ShouldEqual, "preloaded")
	test.That(t, namedImages[1].SourceName, test.ShouldEqual, "color")

	// Should error on invalid source name
	_, _, err = cam.Images(ctx, []string{"not a source"}, nil)
	test.That(t, err, test.ShouldBeError)
	test.That(t, err.Error(), test.ShouldEqual, "invalid source name: not a source")

	namedImages, _, err = cam.Images(ctx, nil, nil)
	test.That(t, err, test.ShouldBeNil)
	cameraImg, err := namedImages[0].Image(ctx)
	test.That(t, err, test.ShouldBeNil)
	preloadedImg, err := getPreloadedImage("pizza")
	test.That(t, err, test.ShouldBeNil)
	diff, _, err := rimage.CompareImages(cameraImg, preloadedImg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diff, test.ShouldEqual, 0)

	err = cam.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
