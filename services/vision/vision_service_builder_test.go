package vision_test

import (
	"context"
	"errors"
	"image"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/data"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	visionObject "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/viscapture"
)

const testCameraName = "camera1"

type (
	simpleDetector   struct{}
	simpleClassifier struct{}
	simpleSegmenter  struct{}
)

func (s *simpleDetector) Detect(ctx context.Context, img image.Image) ([]objectdetection.Detection, error) {
	det1 := objectdetection.NewDetection(image.Rect(0, 0, 50, 50), image.Rect(0, 0, 10, 20), 0.5, "yes")
	return []objectdetection.Detection{det1}, nil
}

func (s *simpleClassifier) Classify(context.Context, image.Image) (classification.Classifications, error) {
	class1 := classification.NewClassification(0.5, "yes")
	return classification.Classifications{class1}, nil
}

func (s *simpleSegmenter) Segment(ctx context.Context, src camera.Camera) ([]*visionObject.Object, error) {
	return []*visionObject.Object{}, nil
}

func TestNewService(t *testing.T) {
	var r inject.Robot
	r.LoggerFunc = func() logging.Logger { return nil }
	var m simpleDetector
	svc, err := vision.DeprecatedNewService(vision.Named("testService"), &r, nil, nil, m.Detect, nil, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}

func TestDefaultCameraSettings(t *testing.T) {
	var r inject.Robot
	var c simpleClassifier
	var d simpleDetector
	var s simpleSegmenter

	fakeCamera := &inject.Camera{
		ImagesFunc: func(
			ctx context.Context,
			filterSourceNames []string,
			extra map[string]interface{},
		) ([]camera.NamedImage, resource.ResponseMetadata, error) {
			sourceImg := image.NewRGBA(image.Rect(0, 0, 3, 3))
			imgBytes, err := rimage.EncodeImage(ctx, sourceImg, utils.MimeTypePNG)
			test.That(t, err, test.ShouldBeNil)
			namedImg, err := camera.NamedImageFromBytes(imgBytes, "", utils.MimeTypePNG, data.Annotations{})
			test.That(t, err, test.ShouldBeNil)
			return []camera.NamedImage{namedImg}, resource.ResponseMetadata{CapturedAt: time.Now()}, nil
		},
	}

	r.LoggerFunc = func() logging.Logger {
		return nil
	}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return fakeCamera, nil
	}
	r.LoggerFunc = func() logging.Logger {
		return logging.NewTestLogger(t)
	}

	svc, err := vision.DeprecatedNewService(vision.Named("testService"), &r, nil, c.Classify, d.Detect, s.Segment, testCameraName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	// test *FromCamera methods with default camera and no camera name
	_, err = svc.DetectionsFromCamera(context.Background(), "", nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.ClassificationsFromCamera(context.Background(), "", 1, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.GetObjectPointClouds(context.Background(), "", nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.CaptureAllFromCamera(context.Background(), "", viscapture.CaptureOptions{}, nil)
	test.That(t, err, test.ShouldBeNil)

	// test *FromCamera methods with no default camera or camera name (should throw error)
	noCameraError := "no camera name provided and no default camera found"

	svc, err = vision.DeprecatedNewService(vision.Named("testService"), &r, nil, c.Classify, d.Detect, s.Segment, "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	_, err = svc.DetectionsFromCamera(context.Background(), "", nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, noCameraError)
	_, err = svc.ClassificationsFromCamera(context.Background(), "", 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, noCameraError)
	_, err = svc.GetObjectPointClouds(context.Background(), "", nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, noCameraError)
	_, err = svc.CaptureAllFromCamera(context.Background(), "", viscapture.CaptureOptions{}, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldEqual, noCameraError)

	// test *FromCamera methods with camera name and default camera (should choose camera name)
	// remove default camera from test robot to ensure that only camera name is used
	secondCameraName := "used-camera"
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		switch name {
		case camera.Named(testCameraName):
			return nil, errors.New("default camera is being used when camera name should instead")
		case camera.Named(secondCameraName):
			return fakeCamera, nil
		default:
			return nil, errors.New("camera not found")
		}
	}
	svc, err = vision.DeprecatedNewService(vision.Named("testService"), &r, nil, c.Classify, d.Detect, s.Segment, testCameraName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	_, err = svc.DetectionsFromCamera(context.Background(), secondCameraName, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.ClassificationsFromCamera(context.Background(), secondCameraName, 1, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.GetObjectPointClouds(context.Background(), secondCameraName, nil)
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.CaptureAllFromCamera(context.Background(), secondCameraName, viscapture.CaptureOptions{}, nil)
	test.That(t, err, test.ShouldBeNil)
}
