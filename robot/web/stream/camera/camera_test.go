package camera_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	camerautils "go.viam.com/rdk/robot/web/stream/camera"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
)

func TestVideoSourceFromCamera(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 3, 3))
	cam := &inject.Camera{
		ImagesFunc: func(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			imgBytes, err := rimage.EncodeImage(ctx, sourceImg, utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			namedImg, err := camera.NamedImageFromBytes(imgBytes, "", utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
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

func TestVideoSourceFromCameraFailure(t *testing.T) {
	malformedCam := &inject.Camera{
		ImagesFunc: func(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			namedImg, err := camera.NamedImageFromBytes([]byte("not a valid image"), "", utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
		},
	}

	vs, err := camerautils.VideoSourceFromCamera(context.Background(), malformedCam)
	test.That(t, err, test.ShouldNotBeNil)
	expectedErrPrefix := "failed to decode lazy encoded image: "
	test.That(t, err.Error(), test.ShouldStartWith, expectedErrPrefix)
	test.That(t, vs, test.ShouldBeNil)
}

func TestVideoSourceFromCameraWithNonsenseMimeType(t *testing.T) {
	sourceImg := image.NewRGBA(image.Rect(0, 0, 3, 3))

	camWithNonsenseMimeType := &inject.Camera{
		ImagesFunc: func(ctx context.Context, filterSourceNames []string, extra map[string]interface{}) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			imgBytes, err := rimage.EncodeImage(ctx, sourceImg, utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			namedImg, err := camera.NamedImageFromBytes(imgBytes, "", "rand")
			test.That(t, err, test.ShouldBeNil)
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{}, nil
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
