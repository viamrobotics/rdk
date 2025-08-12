package camera_test

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

const (
	testCameraName    = "camera1"
	depthCameraName   = "camera_depth"
	failCameraName    = "camera2"
	missingCameraName = "camera3"
	source1Name       = "source1"
	source2Name       = "source2"
)

type simpleSource struct {
	filePath string
}

func (s *simpleSource) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath(s.filePath+".dat.gz"))
	return img, func() {}, err
}

func (s *simpleSource) Close(ctx context.Context) error {
	return nil
}

type simpleSourceWithPCD struct {
	filePath string
}

func (s *simpleSourceWithPCD) Read(ctx context.Context) (image.Image, func(), error) {
	img, err := rimage.NewDepthMapFromFile(
		context.Background(), artifact.MustPath(s.filePath+".dat.gz"))
	return img, func() {}, err
}

func (s *simpleSourceWithPCD) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	return nil, nil
}

func (s *simpleSourceWithPCD) Close(ctx context.Context) error {
	return nil
}

func TestNewPinholeModelWithBrownConradyDistortion(t *testing.T) {
	intrinsics := &transform.PinholeCameraIntrinsics{
		Width:  10,
		Height: 10,
		Fx:     1.0,
		Fy:     2.0,
		Ppx:    3.0,
		Ppy:    4.0,
	}
	distortion := &transform.BrownConrady{}

	expected1 := transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics, Distortion: distortion}
	pinholeCameraModel1 := camera.NewPinholeModelWithBrownConradyDistortion(intrinsics, distortion)
	test.That(t, pinholeCameraModel1, test.ShouldResemble, expected1)

	expected2 := transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics}
	pinholeCameraModel2 := camera.NewPinholeModelWithBrownConradyDistortion(intrinsics, nil)
	test.That(t, pinholeCameraModel2, test.ShouldResemble, expected2)
	test.That(t, pinholeCameraModel2.Distortion, test.ShouldBeNil)

	expected3 := transform.PinholeCameraModel{Distortion: distortion}
	pinholeCameraModel3 := camera.NewPinholeModelWithBrownConradyDistortion(nil, distortion)
	test.That(t, pinholeCameraModel3, test.ShouldResemble, expected3)

	expected4 := transform.PinholeCameraModel{}
	pinholeCameraModel4 := camera.NewPinholeModelWithBrownConradyDistortion(nil, nil)
	test.That(t, pinholeCameraModel4, test.ShouldResemble, expected4)
	test.That(t, pinholeCameraModel4.Distortion, test.ShouldBeNil)
}

func TestNewCamera(t *testing.T) {
	intrinsics1 := &transform.PinholeCameraIntrinsics{Width: 128, Height: 72}
	intrinsics2 := &transform.PinholeCameraIntrinsics{Width: 100, Height: 100}
	videoSrc := &simpleSource{"rimage/board1_small"}
	videoSrcPCD := &simpleSourceWithPCD{"rimage/board1_small"}
	frameRate := float32(10.0)

	// no camera
	_, err := camera.NewVideoSourceFromReader(context.Background(), nil, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeError, errors.New("cannot have a nil reader"))

	// camera with no camera parameters
	cam1, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err := cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeFalse)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)
	test.That(t, props.FrameRate, test.ShouldEqual, 0.0) // test frame rate when it is not set

	cam1, err = camera.NewVideoSourceFromReader(context.Background(), videoSrcPCD, nil, camera.UnspecifiedStream)
	test.That(t, err, test.ShouldBeNil)
	props, err = cam1.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.SupportsPCD, test.ShouldBeTrue)
	test.That(t, props.IntrinsicParams, test.ShouldBeNil)
	props.FrameRate = frameRate
	test.That(t, props.FrameRate, test.ShouldEqual, 10.0) // test frame rate when it is set

	// camera with camera parameters
	cam2, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics1},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	props, err = cam2.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *(props.IntrinsicParams), test.ShouldResemble, *intrinsics1)

	// camera with camera parameters inherited  from other camera
	cam2props, err := cam2.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	cam3, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: cam2props.IntrinsicParams},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	cam3props, err := cam3.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, *(cam3props.IntrinsicParams), test.ShouldResemble, *(cam2props.IntrinsicParams))

	// camera with different camera parameters, will not inherit
	cam4, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: intrinsics2},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	cam4props, err := cam4.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cam4props.IntrinsicParams, test.ShouldNotBeNil)
	test.That(t, *(cam4props.IntrinsicParams), test.ShouldNotResemble, *(cam2props.IntrinsicParams))
}

type cloudSource struct {
	resource.Named
	resource.AlwaysRebuild
	*simpleSource
}

func (cs *cloudSource) NextPointCloud(ctx context.Context) (pointcloud.PointCloud, error) {
	p := pointcloud.NewBasicEmpty()
	return p, p.Set(pointcloud.NewVector(0, 0, 0), nil)
}

func TestCameraWithNoProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	noProj, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	_, err = noProj.NextPointCloud(context.Background())
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// make a camera with a NextPointCloudFunction
	cloudSrc2 := &cloudSource{Named: camera.Named("foo").AsNamed(), simpleSource: videoSrc}
	videoSrc2, err := camera.NewVideoSourceFromReader(context.Background(), cloudSrc2, nil, camera.DepthStream)
	noProj2 := camera.FromVideoSource(resource.NewName(camera.API, "bar"), videoSrc2)
	test.That(t, err, test.ShouldBeNil)
	pc, err := noProj2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, err := camera.DecodeImageFromCamera(context.Background(), rutils.WithLazyMIMEType(rutils.MimeTypePNG), nil, noProj2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, noProj2.Close(context.Background()), test.ShouldBeNil)
}

func TestCameraWithProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	params1 := &transform.PinholeCameraIntrinsics{ // not the real camera parameters -- fake for test
		Width:  1280,
		Height: 720,
		Fx:     200,
		Fy:     200,
		Ppx:    100,
		Ppy:    100,
	}
	src, err := camera.NewVideoSourceFromReader(
		context.Background(),
		videoSrc,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: params1},
		camera.DepthStream,
	)
	test.That(t, err, test.ShouldBeNil)
	pc, err := src.NextPointCloud(context.Background())
	test.That(t, pc.Size(), test.ShouldEqual, 921600)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, src.Close(context.Background()), test.ShouldBeNil)

	// camera with a point cloud function
	cloudSrc2 := &cloudSource{Named: camera.Named("foo").AsNamed(), simpleSource: videoSrc}
	props, err := src.Properties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	videoSrc2, err := camera.NewVideoSourceFromReader(
		context.Background(),
		cloudSrc2,
		&transform.PinholeCameraModel{PinholeCameraIntrinsics: props.IntrinsicParams},
		camera.DepthStream,
	)
	cam2 := camera.FromVideoSource(resource.NewName(camera.API, "bar"), videoSrc2)
	test.That(t, err, test.ShouldBeNil)
	pc, err = videoSrc2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, err := camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypePNG, nil, cam2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)
	// cam2 should implement a default GetImages, that just returns the one image
	images, _, err := videoSrc2.Images(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(images), test.ShouldEqual, 1)
	test.That(t, images[0].Image, test.ShouldHaveSameTypeAs, &rimage.DepthMap{})
	test.That(t, images[0].Image.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, images[0].Image.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
}

// verifyImageEquality compares two images and verifies they are identical.
func verifyImageEquality(t *testing.T, img1, img2 image.Image) {
	t.Helper()
	diff, _, err := rimage.CompareImages(img1, img2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diff, test.ShouldEqual, 0)
}

// verifyDecodedImage verifies that decoded image bytes match the original image.
func verifyDecodedImage(t *testing.T, imgBytes []byte, mimeType string, originalImg image.Image) {
	t.Helper()
	test.That(t, len(imgBytes), test.ShouldBeGreaterThan, 0)

	// For JPEG, compare the raw bytes instead of the decoded image since the decoded image is
	// not guaranteed to be the same as the original image due to lossy compression.
	if mimeType == rutils.MimeTypeJPEG {
		expectedBytes, err := rimage.EncodeImage(context.Background(), originalImg, mimeType)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, imgBytes, test.ShouldResemble, expectedBytes)
		return
	}

	// For other formats, compare the decoded images
	decodedImg, err := rimage.DecodeImage(context.Background(), imgBytes, mimeType)
	test.That(t, err, test.ShouldBeNil)
	verifyImageEquality(t, decodedImg, originalImg)
}

func TestGetImageFromGetImages(t *testing.T) {
	testImg1 := image.NewRGBA(image.Rect(0, 0, 100, 100))
	testImg2 := image.NewRGBA(image.Rect(0, 0, 200, 200))

	rgbaCam := inject.NewCamera("rgba_cam")
	rgbaCam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		return []camera.NamedImage{
			{Image: testImg1, SourceName: source1Name},
			{Image: testImg2, SourceName: source2Name},
		}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}

	dm := rimage.NewEmptyDepthMap(100, 100)
	depthCam := inject.NewCamera("depth_cam")
	depthCam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		return []camera.NamedImage{{Image: dm, SourceName: source1Name}}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}

	t.Run("PNG mime type", func(t *testing.T) {
		imgBytes, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypePNG, rgbaCam, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypePNG)
		verifyDecodedImage(t, imgBytes, rutils.MimeTypePNG, testImg1)
	})

	t.Run("JPEG mime type", func(t *testing.T) {
		imgBytes, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypeJPEG, rgbaCam, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypeJPEG)
		verifyDecodedImage(t, imgBytes, rutils.MimeTypeJPEG, testImg1)
	})

	t.Run("request mime type depth, but actual image is RGBA", func(t *testing.T) {
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypeRawDepth, rgbaCam, nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "cannot convert image type")
	})

	t.Run("request JPEG, but actual image is depth map", func(t *testing.T) {
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypeJPEG, depthCam, nil)
		test.That(t, err, test.ShouldBeNil) // expect success because we can convert the depth map to JPEG
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypeJPEG)
		verifyDecodedImage(t, img, rutils.MimeTypeJPEG, dm)
	})

	t.Run("request PNG, but actual image is depth map", func(t *testing.T) {
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypePNG, depthCam, nil)
		test.That(t, err, test.ShouldBeNil) // expect success because we can convert the depth map to PNG
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypePNG)
		verifyDecodedImage(t, img, rutils.MimeTypePNG, dm)
	})

	t.Run("request empty mime type", func(t *testing.T) {
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, "", rgbaCam, nil)
		// empty mime type defaults to JPEG
		test.That(t, err, test.ShouldBeNil)
		test.That(t, img, test.ShouldNotBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypeJPEG)
		verifyDecodedImage(t, img, rutils.MimeTypeJPEG, testImg1)
	})

	t.Run("error case", func(t *testing.T) {
		errorCam := inject.NewCamera("error_cam")
		errorCam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return nil, resource.ResponseMetadata{}, errors.New("test error")
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypePNG, errorCam, nil)
		test.That(t, err, test.ShouldBeError, errors.New("could not get images from camera: test error"))
	})

	t.Run("empty images case", func(t *testing.T) {
		emptyCam := inject.NewCamera("empty_cam")
		emptyCam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypePNG, emptyCam, nil)
		test.That(t, err, test.ShouldBeError, errors.New("no images returned from camera"))
	})

	t.Run("nil image case", func(t *testing.T) {
		nilImageCam := inject.NewCamera("nil_image_cam")
		nilImageCam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{{Image: nil, SourceName: source1Name}}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, rutils.MimeTypePNG, nilImageCam, nil)
		test.That(t, err, test.ShouldBeError, errors.New("image is nil"))
	})

	t.Run("multiple images, specify source name", func(t *testing.T) {
		sourceName := source2Name
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), &sourceName, rutils.MimeTypePNG, rgbaCam, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypePNG)
		verifyDecodedImage(t, img, rutils.MimeTypePNG, testImg2)
	})
}

func TestGetImagesFromGetImage(t *testing.T) {
	logger := logging.NewTestLogger(t)
	testImg := image.NewRGBA(image.Rect(0, 0, 100, 100))

	rgbaCam := inject.NewCamera("rgba_cam")
	rgbaCam.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
		imgBytes, err := rimage.EncodeImage(ctx, testImg, mimeType)
		if err != nil {
			return nil, camera.ImageMetadata{}, err
		}
		return imgBytes, camera.ImageMetadata{MimeType: mimeType}, nil
	}

	t.Run("PNG mime type", func(t *testing.T) {
		startTime := time.Now()
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, rgbaCam, logger)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		verifyImageEquality(t, images[0].Image, testImg)
		test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, metadata.CapturedAt.After(startTime), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.Before(endTime), test.ShouldBeTrue)
	})

	t.Run("JPEG mime type", func(t *testing.T) {
		startTime := time.Now()
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypeJPEG, rgbaCam, logger)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		imgBytes, err := rimage.EncodeImage(context.Background(), images[0].Image, rutils.MimeTypeJPEG)
		test.That(t, err, test.ShouldBeNil)
		verifyDecodedImage(t, imgBytes, rutils.MimeTypeJPEG, testImg)
		test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, metadata.CapturedAt.After(startTime), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.Before(endTime), test.ShouldBeTrue)
	})

	t.Run("request mime type depth, but actual image is RGBA", func(t *testing.T) {
		rgbaImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
		rgbaCam := inject.NewCamera("rgba_cam")
		rgbaCam.ImageFunc = func(ctx context.Context, reqMimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			imgBytes, err := rimage.EncodeImage(ctx, rgbaImg, rutils.MimeTypeRawRGBA)
			if err != nil {
				return nil, camera.ImageMetadata{}, err
			}
			return imgBytes, camera.ImageMetadata{MimeType: rutils.MimeTypeRawRGBA}, nil
		}
		startTime := time.Now()
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypeRawDepth, rgbaCam, logger)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, metadata.CapturedAt.After(startTime), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.Before(endTime), test.ShouldBeTrue)
		verifyImageEquality(t, images[0].Image, rgbaImg) // we should ignore the requested mime type and get back an RGBA image
	})

	t.Run("error case", func(t *testing.T) {
		errorCam := inject.NewCamera("error_cam")
		errorCam.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			return nil, camera.ImageMetadata{}, errors.New("test error")
		}
		_, _, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, errorCam, logger)
		test.That(t, err, test.ShouldBeError, errors.New("could not get image bytes from camera: test error"))
	})

	t.Run("empty bytes case", func(t *testing.T) {
		emptyCam := inject.NewCamera("empty_cam")
		emptyCam.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			return []byte{}, camera.ImageMetadata{MimeType: mimeType}, nil
		}
		_, _, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, emptyCam, logger)
		test.That(t, err, test.ShouldBeError, errors.New("received empty bytes from camera"))
	})
}

func TestImagesExtraParam(t *testing.T) {
	ctx := context.Background()

	smallImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	largeImg := image.NewRGBA(image.Rect(0, 0, 20, 20))

	cam := inject.NewCamera("extra_param_cam")
	cam.ImagesFunc = func(ctx context.Context, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		size := "small"
		if extra != nil {
			if s, ok := extra["size"].(string); ok {
				size = s
			}
		}

		var img image.Image
		switch size {
		case "large":
			img = largeImg
		default:
			img = smallImg
		}

		return []camera.NamedImage{{Image: img, SourceName: source1Name}}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}

	cases := []struct {
		name           string
		extra          map[string]interface{}
		expectedWidth  int
		expectedHeight int
	}{
		{name: "large via extra", extra: map[string]interface{}{"size": "large"}, expectedWidth: 20, expectedHeight: 20},
		{name: "small via extra", extra: map[string]interface{}{"size": "small"}, expectedWidth: 10, expectedHeight: 10},
		{name: "default size when extra nil", extra: nil, expectedWidth: 10, expectedHeight: 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			images, _, err := cam.Images(ctx, tc.extra)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(images), test.ShouldEqual, 1)
			test.That(t, images[0].Image.Bounds().Dx(), test.ShouldEqual, tc.expectedWidth)
			test.That(t, images[0].Image.Bounds().Dy(), test.ShouldEqual, tc.expectedHeight)
		})
	}
}
