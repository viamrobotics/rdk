package vision_test

import (
	"context"
	"image"
	"testing"

	"github.com/pkg/errors"
	// register cameras for testing.

	pb "go.viam.com/api/service/vision/v1"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"
	"go.viam.com/utils/protoutils"

	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/camera/register"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
)

func newServer(m map[resource.Name]interface{}) (pb.VisionServiceServer, error) {
	svc, err := subtype.New(m)
	if err != nil {
		return nil, err
	}
	return vision.NewServer(svc), nil
}

func TestVisionServerFailures(t *testing.T) {
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	imgBytes, err := rimage.EncodeImage(context.Background(), img, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	detectRequest := &pb.GetDetectionsRequest{
		Name:     testVisionServiceName,
		Image:    imgBytes,
		Width:    int64(img.Width()),
		Height:   int64(img.Height()),
		MimeType: utils.MimeTypeJPEG,
	}
	// no service
	m := map[resource.Name]interface{}{}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetDetections(context.Background(), detectRequest)
	test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:service:vision/vision1\" not found"))

	// set up the robot with something that is not a vision service
	m = map[resource.Name]interface{}{vision.Named(testVisionServiceName): "not what you want"}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetDetections(context.Background(), detectRequest)
	test.That(t, err, test.ShouldBeError, vision.NewUnimplementedInterfaceError("string"))

	// correct server with error returned
	injectVS := &inject.VisionService{}
	injectCamera := &inject.Camera{}
	passedErr := errors.New("fake error")
	injectVS.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		return nil, passedErr
	}
	m = map[resource.Name]interface{}{
		vision.Named(testVisionServiceName):  injectVS,
		vision.Named(testVisionServiceName2): injectVS,
		camera.Named("faked_camera"):         injectCamera,
	}
	server, err = newServer(m)
	test.That(t, err, test.ShouldBeNil)
	_, err = server.GetDetections(context.Background(), detectRequest)
	test.That(t, err, test.ShouldBeError, passedErr)
}

func TestServerGetDetections(t *testing.T) {
	injectVS := &inject.VisionService{}
	m := map[resource.Name]interface{}{
		vision.Named(testVisionServiceName): injectVS,
	}
	server, err := newServer(m)
	test.That(t, err, test.ShouldBeNil)

	// returns response
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/tflite/dogscute.jpeg"))
	test.That(t, err, test.ShouldBeNil)
	imgBytes, err := rimage.EncodeImage(context.Background(), img, utils.MimeTypeJPEG)
	test.That(t, err, test.ShouldBeNil)
	extra := map[string]interface{}{"foo": "GetDetections"}
	ext, err := protoutils.StructToStructPb(extra)
	detectRequest := &pb.GetDetectionsRequest{
		Name:     testVisionServiceName,
		Image:    imgBytes,
		Width:    int64(img.Width()),
		Height:   int64(img.Height()),
		MimeType: utils.MimeTypeJPEG,
		Extra:    ext,
	}
	injectVS.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rectangle{}, 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	test.That(t, err, test.ShouldBeNil)
	resp, err := server.GetDetections(context.Background(), detectRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(resp.GetDetections()), test.ShouldEqual, 1)
	test.That(t, resp.GetDetections()[0].GetClassName(), test.ShouldEqual, "yes")
}
