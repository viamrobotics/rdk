package vision_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/test"
)

const (
	testVisionServiceName = "vision1"
	testVisionServiceName = "vision2"
)

func TestFromRobot(t *testing.T) {
	svc1 := &inject.VisionService{}
	sv1.DetectionFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rectangle{}, 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	svc, err := vision.FromRobot(r, testVisionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Backgroun(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}

func TestNewService(t *testing.T) {
	r := &inject.Robot{}
	simpleDetector := func(context.Context, image.Image) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rectangle{}, 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	svc, err := vision.NewService("testService", simpleDetector, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Backgroun(), nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}
