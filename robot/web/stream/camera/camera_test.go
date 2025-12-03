package camera_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/gostream"
	"go.viam.com/rdk/rimage"
	camerautils "go.viam.com/rdk/robot/web/stream/camera"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestVideoSourceFromCamera(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 3, 3))
	cam := &inject.Camera{
		ImageFunc: func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			imgBytes, err := rimage.EncodeImage(ctx, sourceImg, utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			return imgBytes, camera.ImageMetadata{MimeType: utils.MimeTypePNG}, nil
		},
	}
	vs, err := camerautils.VideoSourceFromCamera(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	stream, err := vs.Stream(context.Background())
	test.That(t, err, test.ShouldBeNil)

	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)

	diffVal, _, err := rimage.CompareImages(img, sourceImg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, diffVal, test.ShouldEqual, 0)
}

func TestVideoSourceFromCameraFalsyVideoProps(t *testing.T) {
	malformedCam := &inject.Camera{
		ImageFunc: func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			return []byte("not a valid image"), camera.ImageMetadata{MimeType: utils.MimeTypePNG}, nil
		},
	}

	// VideoSourceFromCamera should not fail even with a malformed camera,
	// since we no longer call GetImage during the conversion.
	// See: https://viam.atlassian.net/browse/RSDK-12744
	//
	// Instead, it should return a VideoSource with empty video props.
	vs, err := camerautils.VideoSourceFromCamera(context.Background(), malformedCam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	propProvider, ok := vs.(gostream.VideoPropertyProvider)
	test.That(t, ok, test.ShouldBeTrue)
	props, err := propProvider.MediaProperties(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, props.Width, test.ShouldEqual, 0)
	test.That(t, props.Height, test.ShouldEqual, 0)
}

func TestVideoSourceFromCameraWithNonsenseMimeType(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 3, 3))

	camWithNonsenseMimeType := &inject.Camera{
		ImageFunc: func(ctx context.Context, mimeType string, extra map[string]interface{}) ([]byte, camera.ImageMetadata, error) {
			imgBytes, err := rimage.EncodeImage(ctx, sourceImg, utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			return imgBytes, camera.ImageMetadata{MimeType: "rand"}, nil
		},
	}

	// Should log a warning though due to the nonsense MIME type
	vs, err := camerautils.VideoSourceFromCamera(context.Background(), camWithNonsenseMimeType)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, vs, test.ShouldNotBeNil)

	stream, _ := vs.Stream(context.Background())
	img, _, err := stream.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img, test.ShouldNotBeNil)
}
