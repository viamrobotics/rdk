//go:build !no_media

package vision_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
)

const (
	testVisionServiceName  = "vision1"
	testVisionServiceName2 = "vision2"
)

func TestFromRobot(t *testing.T) {
	svc1 := &inject.VisionService{}
	svc1.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rectangle{}, 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	var r inject.Robot
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return svc1, nil
	}
	svc, err := vision.FromRobot(&r, testVisionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}

type simpleDetector struct{}

func (s *simpleDetector) Detect(context.Context, image.Image) ([]objectdetection.Detection, error) {
	det1 := objectdetection.NewDetection(image.Rectangle{}, 0.5, "yes")
	return []objectdetection.Detection{det1}, nil
}

func TestNewService(t *testing.T) {
	var r inject.Robot
	var m simpleDetector
	svc, err := vision.NewService(vision.Named("testService"), &r, nil, nil, m.Detect, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}
