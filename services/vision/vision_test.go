package vision_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision/objectdetection"
)

const (
	testVisionServiceName  = "vision1" // Used both here and server_test.go
	testVisionServiceName2 = "vision2" // Used in server_test.go, but not here
)

func TestFromRobot(t *testing.T) {
	svc1 := &inject.VisionService{}
	svc1.DetectionsFunc = func(ctx context.Context, img image.Image, extra map[string]interface{}) ([]objectdetection.Detection, error) {
		det1 := objectdetection.NewDetection(image.Rect(0, 0, 50, 50), image.Rect(0, 0, 10, 20), 0.5, "yes")
		return []objectdetection.Detection{det1}, nil
	}
	var r inject.Robot
	r.LoggerFunc = func() logging.Logger {
		return nil
	}
	r.ResourceByNameFunc = func(name resource.Name) (resource.Resource, error) {
		return svc1, nil
	}
	svc, err := vision.FromProvider(&r, testVisionServiceName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)
	result, err := svc.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(result), test.ShouldEqual, 1)
	test.That(t, result[0].Score(), test.ShouldEqual, 0.5)
}
