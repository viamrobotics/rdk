package camera_test

import (
	"context"
	"fmt"
	"image"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
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
	source3Name       = "source3"
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

func (s *simpleSourceWithPCD) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
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

func (cs *cloudSource) NextPointCloud(ctx context.Context, extra map[string]interface{}) (pointcloud.PointCloud, error) {
	p := pointcloud.NewBasicEmpty()
	return p, p.Set(pointcloud.NewVector(0, 0, 0), nil)
}

func TestCameraWithNoProjector(t *testing.T) {
	videoSrc := &simpleSource{"rimage/board1"}
	noProj, err := camera.NewVideoSourceFromReader(context.Background(), videoSrc, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	_, err = noProj.NextPointCloud(context.Background(), nil)
	test.That(t, errors.Is(err, transform.ErrNoIntrinsics), test.ShouldBeTrue)

	// make a camera with a NextPointCloudFunction
	cloudSrc2 := &cloudSource{Named: camera.Named("foo").AsNamed(), simpleSource: videoSrc}
	videoSrc2, err := camera.NewVideoSourceFromReader(context.Background(), cloudSrc2, nil, camera.DepthStream)
	noProj2 := camera.FromVideoSource(resource.NewName(camera.API, "bar"), videoSrc2)
	test.That(t, err, test.ShouldBeNil)
	pc, err := noProj2.NextPointCloud(context.Background(), nil)
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
	pc, err := src.NextPointCloud(context.Background(), nil)
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
	pc, err = videoSrc2.NextPointCloud(context.Background(), nil)
	test.That(t, err, test.ShouldBeNil)
	_, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)

	img, err := camera.DecodeImageFromCamera(context.Background(), rutils.MimeTypePNG, nil, cam2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, img.Bounds().Dy(), test.ShouldEqual, 720)
	// cam2 should implement a default GetImages, that just returns the one image
	images, _, err := videoSrc2.Images(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(images), test.ShouldEqual, 1)
	imgFromImages, err := images[0].Image(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, imgFromImages, test.ShouldHaveSameTypeAs, &rimage.DepthMap{})
	test.That(t, imgFromImages.Bounds().Dx(), test.ShouldEqual, 1280)
	test.That(t, imgFromImages.Bounds().Dy(), test.ShouldEqual, 720)

	test.That(t, cam2.Close(context.Background()), test.ShouldBeNil)
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
	test.That(t, rimage.ImagesExactlyEqual(decodedImg, originalImg), test.ShouldBeTrue)
}

func TestGetImageFromGetImages(t *testing.T) {
	testImg1 := image.NewRGBA(image.Rect(0, 0, 100, 100))
	testImg2 := image.NewRGBA(image.Rect(0, 0, 200, 200))
	annotations1 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "object1"}}}
	annotations2 := data.Annotations{Classifications: []data.Classification{{Label: "object2"}}}

	rgbaCam := inject.NewCamera("rgba_cam")
	rgbaCam.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		namedImg1, err := camera.NamedImageFromImage(testImg1, source1Name, rutils.MimeTypeRawRGBA, annotations1)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		namedImg2, err := camera.NamedImageFromImage(testImg2, source2Name, rutils.MimeTypeRawRGBA, annotations2)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		return []camera.NamedImage{
			namedImg1,
			namedImg2,
		}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}

	dm := rimage.NewEmptyDepthMap(100, 100)
	depthCam := inject.NewCamera("depth_cam")
	depthCam.ImagesFunc = func(
		ctx context.Context,
		filterSourceNames []string,
		extra map[string]interface{},
	) ([]camera.NamedImage, resource.ResponseMetadata, error) {
		namedImg, err := camera.NamedImageFromImage(dm, source1Name, rutils.MimeTypeRawDepth, annotations1)
		if err != nil {
			return nil, resource.ResponseMetadata{}, err
		}
		return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
	}

	t.Run("request empty mime type", func(t *testing.T) {
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), nil, rgbaCam, nil, nil)
		// empty mime type defaults to JPEG
		test.That(t, err, test.ShouldBeNil)
		test.That(t, img, test.ShouldNotBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypeRawRGBA)
		test.That(t, metadata.Annotations, test.ShouldResemble, annotations1)
		verifyDecodedImage(t, img, rutils.MimeTypeRawRGBA, testImg1)
	})

	t.Run("error case", func(t *testing.T) {
		errorCam := inject.NewCamera("error_cam")
		errorCam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return nil, resource.ResponseMetadata{}, errors.New("test error")
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, errorCam, nil, nil)
		test.That(t, err, test.ShouldBeError, errors.New("could not get images from camera: test error"))
	})

	t.Run("empty images case", func(t *testing.T) {
		emptyCam := inject.NewCamera("empty_cam")
		emptyCam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			return []camera.NamedImage{}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, emptyCam, nil, nil)
		test.That(t, err, test.ShouldBeError, errors.New("no images returned from camera"))
	})

	t.Run("nil image case", func(t *testing.T) {
		nilImageCam := inject.NewCamera("nil_image_cam")
		nilImageCam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			namedImg, err := camera.NamedImageFromImage(nil, source1Name, rutils.MimeTypeRawRGBA, data.Annotations{})
			if err != nil {
				return nil, resource.ResponseMetadata{}, err
			}
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}
		_, _, err := camera.GetImageFromGetImages(context.Background(), nil, nilImageCam, nil, nil)
		test.That(t, err, test.ShouldBeError,
			errors.New("could not get images from camera: must provide image to construct a named image from image"),
		)
	})

	t.Run("multiple images, specify source name", func(t *testing.T) {
		sourceName := source2Name
		img, metadata, err := camera.GetImageFromGetImages(context.Background(), &sourceName, rgbaCam, nil, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, metadata.MimeType, test.ShouldEqual, rutils.MimeTypeRawRGBA)
		verifyDecodedImage(t, img, rutils.MimeTypeRawRGBA, testImg2)
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
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, rgbaCam, logger, nil)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		img, err := images[0].Image(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, metadata.CapturedAt.After(startTime), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.Before(endTime), test.ShouldBeTrue)
	})

	t.Run("JPEG mime type", func(t *testing.T) {
		startTime := time.Now()
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypeJPEG, rgbaCam, logger, nil)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		img, err := images[0].Image(context.Background())
		test.That(t, err, test.ShouldBeNil)
		imgBytes, err := rimage.EncodeImage(context.Background(), img, rutils.MimeTypeJPEG)
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
		images, metadata, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypeRawDepth, rgbaCam, logger, nil)
		endTime := time.Now()
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(images), test.ShouldEqual, 1)
		test.That(t, images[0].SourceName, test.ShouldEqual, "")
		test.That(t, metadata.CapturedAt.IsZero(), test.ShouldBeFalse)
		test.That(t, metadata.CapturedAt.After(startTime), test.ShouldBeTrue)
		test.That(t, metadata.CapturedAt.Before(endTime), test.ShouldBeTrue)
		img, err := images[0].Image(context.Background())
		test.That(t, err, test.ShouldBeNil)
		test.That(t, rimage.ImagesExactlyEqual(img, rgbaImg), test.ShouldBeTrue)
	})

	t.Run("error case", func(t *testing.T) {
		errorCam := inject.NewCamera("error_cam")
		errorCam.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			return nil, camera.ImageMetadata{}, errors.New("test error")
		}
		_, _, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, errorCam, logger, nil)
		test.That(t, err, test.ShouldBeError, errors.New("could not get image bytes from camera: test error"))
	})

	t.Run("empty bytes case", func(t *testing.T) {
		emptyCam := inject.NewCamera("empty_cam")
		emptyCam.ImageFunc = func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			return []byte{}, camera.ImageMetadata{MimeType: mimeType}, nil
		}
		_, _, err := camera.GetImagesFromGetImage(context.Background(), rutils.MimeTypePNG, emptyCam, logger, nil)
		test.That(t, err, test.ShouldBeError, errors.New("received empty bytes from camera"))
	})
}

// TestImages asserts the core expected behavior of the Images API.
func TestImages(t *testing.T) {
	ctx := context.Background()
	t.Run("extra param", func(t *testing.T) {
		respImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
		annotations1 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "annotation1"}}}

		cam := inject.NewCamera("extra_param_cam")
		cam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(extra) == 0 {
				return nil, resource.ResponseMetadata{}, fmt.Errorf("extra parameters required")
			}
			namedImg, err := camera.NamedImageFromImage(respImg, source1Name, rutils.MimeTypeRawRGBA, annotations1)
			if err != nil {
				return nil, resource.ResponseMetadata{}, err
			}
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		}

		t.Run("success with extra params", func(t *testing.T) {
			images, _, err := cam.Images(ctx, nil, map[string]interface{}{"param": "value"})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(images), test.ShouldEqual, 1)
			img, err := images[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, respImg), test.ShouldBeTrue)
			test.That(t, images[0].SourceName, test.ShouldEqual, source1Name)
			test.That(t, images[0].Annotations, test.ShouldResemble, annotations1)
		})

		t.Run("error when no extra params", func(t *testing.T) {
			_, _, err := cam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldNotBeNil)
			test.That(t, err.Error(), test.ShouldEqual, "extra parameters required")
		})
	})

	t.Run("filter source names", func(t *testing.T) {
		img1 := image.NewRGBA(image.Rect(0, 0, 10, 10))
		img2 := rimage.NewEmptyDepthMap(10, 10)
		img3 := image.NewNRGBA(image.Rect(0, 0, 30, 30))

		annotations1 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "object1"}}}
		annotations2 := data.Annotations{Classifications: []data.Classification{{Label: "object2"}}}
		annotations3 := data.Annotations{BoundingBoxes: []data.BoundingBox{{Label: "object3"}}}

		namedImg1, err := camera.NamedImageFromImage(img1, source1Name, rutils.MimeTypePNG, annotations1)
		test.That(t, err, test.ShouldBeNil)
		namedImg2, err := camera.NamedImageFromImage(img2, source2Name, rutils.MimeTypeRawDepth, annotations2)
		test.That(t, err, test.ShouldBeNil)
		namedImg3, err := camera.NamedImageFromImage(img3, source3Name, rutils.MimeTypeJPEG, annotations3)
		test.That(t, err, test.ShouldBeNil)

		allImgs := []camera.NamedImage{namedImg1, namedImg2, namedImg3}
		availableSources := map[string]camera.NamedImage{
			source1Name: namedImg1,
			source2Name: namedImg2,
			source3Name: namedImg3,
		}

		cam := inject.NewCamera("multiple_sources_cam")
		cam.ImagesFunc = func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			if len(filterSourceNames) == 0 {
				return allImgs, resource.ResponseMetadata{}, nil
			}

			var result []camera.NamedImage
			for _, sourceName := range filterSourceNames {
				if img, ok := availableSources[sourceName]; ok {
					result = append(result, img)
				} else {
					return nil, resource.ResponseMetadata{}, fmt.Errorf("requested source name not found: %s", sourceName)
				}
			}
			return result, resource.ResponseMetadata{}, nil
		}

		t.Run("nil filter returns all sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, nil, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 3)

			returnedSources := make(map[string]bool)
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, len(returnedSources), test.ShouldEqual, 3)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source2Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
		})

		t.Run("empty filter returns all sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 3)

			returnedSources := make(map[string]bool)
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, len(returnedSources), test.ShouldEqual, 3)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source2Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
		})

		t.Run("single valid source", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{source2Name}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 1)
			test.That(t, imgs[0].SourceName, test.ShouldEqual, source2Name)
			test.That(t, imgs[0].MimeType(), test.ShouldEqual, rutils.MimeTypeRawDepth)
			test.That(t, imgs[0].Annotations, test.ShouldResemble, annotations2)
			img, err := imgs[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img2), test.ShouldBeTrue)
		})

		t.Run("multiple valid sources", func(t *testing.T) {
			imgs, _, err := cam.Images(ctx, []string{source3Name, source1Name}, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(imgs), test.ShouldEqual, 2)
			returnedSources := map[string]bool{}
			for _, img := range imgs {
				returnedSources[img.SourceName] = true
			}
			test.That(t, returnedSources[source3Name], test.ShouldBeTrue)
			test.That(t, returnedSources[source1Name], test.ShouldBeTrue)

			test.That(t, imgs[0].MimeType(), test.ShouldEqual, rutils.MimeTypeJPEG)
			test.That(t, imgs[0].Annotations, test.ShouldResemble, annotations3)
			img, err := imgs[0].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img3), test.ShouldBeTrue)

			test.That(t, imgs[1].MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, imgs[1].Annotations, test.ShouldResemble, annotations1)
			img, err = imgs[1].Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, img1), test.ShouldBeTrue)
		})

		t.Run("single invalid source", func(t *testing.T) {
			_, _, err := cam.Images(ctx, []string{"invalid_source"}, nil)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "requested source name not found: invalid_source")
		})

		t.Run("mix of valid and invalid sources", func(t *testing.T) {
			_, _, err := cam.Images(ctx, []string{source1Name, "invalid_source"}, nil)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "requested source name not found: invalid_source")
		})
	})
}

func TestNamedImage(t *testing.T) {
	ctx := context.Background()
	testImg := image.NewRGBA(image.Rect(0, 0, 10, 10))
	testImgPNGBytes, err := rimage.EncodeImage(ctx, testImg, rutils.MimeTypePNG)
	test.That(t, err, test.ShouldBeNil)
	testImgJPEGBytes, err := rimage.EncodeImage(ctx, testImg, rutils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	badBytes := []byte("trust bro i'm an image ong")
	sourceName := "test_source"
	annotations := data.Annotations{
		BoundingBoxes: []data.BoundingBox{
			{Label: "object1"},
		},
	}
	t.Run("NamedImageFromBytes", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, ni.Annotations, test.ShouldResemble, annotations)
		})
		t.Run("error on nil data", func(t *testing.T) {
			_, err := camera.NamedImageFromBytes(nil, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeError, errors.New("must provide image bytes to construct a named image from bytes"))
		})
		t.Run("error on empty mime type", func(t *testing.T) {
			_, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, "", annotations)
			test.That(t, err, test.ShouldBeError, errors.New("must provide a mime type to construct a named image"))
		})
	})

	t.Run("NamedImageFromImage", func(t *testing.T) {
		t.Run("success", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
			test.That(t, ni.Annotations, test.ShouldResemble, annotations)
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)
		})
		t.Run("error on nil image", func(t *testing.T) {
			_, err := camera.NamedImageFromImage(nil, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeError, errors.New("must provide image to construct a named image from image"))
		})
		t.Run("defaults to JPEG when mime type is empty", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, "", annotations)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, ni.SourceName, test.ShouldEqual, sourceName)
			test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypeJPEG)

			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgJPEGBytes)
		})
	})

	t.Run("Image method", func(t *testing.T) {
		t.Run("when image is already populated, it should return the image and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)

			// should return the same image instance
			img2, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, reflect.ValueOf(img).Pointer(), test.ShouldEqual, reflect.ValueOf(img2).Pointer())
		})

		t.Run("when only data is populated, it should decode the data and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)

			// first call should decode
			img, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img, testImg), test.ShouldBeTrue)

			// second call should return cached image
			img2, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, rimage.ImagesExactlyEqual(img2, testImg), test.ShouldBeTrue)
			test.That(t, reflect.ValueOf(img).Pointer(), test.ShouldEqual, reflect.ValueOf(img2).Pointer())
		})

		t.Run("error when neither image nor data is populated", func(t *testing.T) {
			var ni camera.NamedImage
			_, err := ni.Image(ctx)
			test.That(t, err, test.ShouldBeError, errors.New("no image or image bytes available"))
		})

		t.Run("error when data is invalid", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(badBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "could not decode image config: image: unknown format")
		})

		t.Run("error when mime type mismatches and decode fails", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgJPEGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err, test.ShouldWrap, camera.ErrMIMETypeBytesMismatch)
		})

		t.Run("error when decode fails for other reasons", func(t *testing.T) {
			corruptedPNGBytes := append([]byte(nil), testImgPNGBytes...)
			corruptedPNGBytes[len(corruptedPNGBytes)-5] = 0 // corrupt it

			ni, err := camera.NamedImageFromBytes(corruptedPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Image(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual, "could not decode bytes into image.Image: png: invalid format: invalid checksum")
		})
	})

	t.Run("Bytes method", func(t *testing.T) {
		t.Run("when data is already populated, it should return the data and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)
			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgPNGBytes)

			// should return the same data instance
			data2, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, &data[0], test.ShouldEqual, &data2[0])
		})

		t.Run("when only image is populated, it should encode the image and cache it", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, rutils.MimeTypePNG, annotations)
			test.That(t, err, test.ShouldBeNil)

			// first call should encode
			data, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data, test.ShouldResemble, testImgPNGBytes)

			// second call should return cached data
			data2, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, data2, test.ShouldResemble, testImgPNGBytes)
			test.That(t, &data[0], test.ShouldEqual, &data2[0])
		})

		t.Run("error when neither image nor data is populated", func(t *testing.T) {
			var ni camera.NamedImage
			_, err := ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeError, errors.New("no image or image bytes available"))
		})

		t.Run("error when encoding fails", func(t *testing.T) {
			ni, err := camera.NamedImageFromImage(testImg, sourceName, "bad-mime-type", annotations)
			test.That(t, err, test.ShouldBeNil)
			_, err = ni.Bytes(ctx)
			test.That(t, err, test.ShouldBeError)
			test.That(t, err.Error(), test.ShouldEqual,
				`could not encode image with encoding bad-mime-type: do not know how to encode "bad-mime-type"`)
		})
	})

	t.Run("MimeType method", func(t *testing.T) {
		ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.MimeType(), test.ShouldEqual, rutils.MimeTypePNG)
	})

	t.Run("Annotations method", func(t *testing.T) {
		ni, err := camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, annotations)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.Annotations, test.ShouldResemble, annotations)

		ni, err = camera.NamedImageFromBytes(testImgPNGBytes, sourceName, rutils.MimeTypePNG, data.Annotations{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ni.Annotations.Empty(), test.ShouldBeTrue)
	})
}
