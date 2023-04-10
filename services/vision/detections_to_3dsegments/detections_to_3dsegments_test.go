package detectionstosegments

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/camera"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/objectdetection"
	"go.viam.com/rdk/vision/segmentation"
	"go.viam.com/test"
)

type simpleDetector struct{}

func (s *simpleDetector) Detect(context.Context, image.Image) ([]objectdetection.Detection, error) {
	det1 := objectdetection.NewDetection(image.Rect(10, 10, 20, 20), 0.5, "yes")
	return []objectdetection.Detection{det1}, nil
}

func Test3DSegmentsFromDetector(t *testing.T) {
	logger := golog.NewTestLogger(t)
	r := &inject.Robot{}
	svc, err := vision.NewService("testDetector", &simpleDetector{}, r)
	test.That(t, err, test.ShouldBeNil)
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return nil, errors.New("no pointcloud")
	}
	cam.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return &transform.ParallelProjection{}, nil
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("fakeCamera"), vision.Named("testDetector")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "fakeCamera":
			return cam, nil
		case "testDetector":
			return svc, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}
	params := &segmentation.DetectionSegmenterConfig{
		DetectorName:     "testDetector",
		ConfidenceThresh: 0.2,
	}
	// bad registration, no parameters
	_, err = register3DSegmenterFromDetector(context.Background(), "test_seg", nil, r, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "cannot be nil")
	// bad registration, no such detector
	params.DetectorName = "noDetector"
	_, err = register3DSegmenterFromDetector(context.Background(), "test_seg", params, r, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "could not find necessary dependency")
	// successful registration
	params.DetectorName = "testDetector"
	seg, err := register3DSegmenterFromDetector(context.Background(), "test_rcs", params, r, logger)
	test.That(t, err, test.ShouldBeNil)

	// fails on not finding camera
	_, err = seg.GetObjectPointClouds(context.Background(), "no_camera", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")

	// fails since camera cannot generate point clouds
	_, err = seg.GetObjectPointClouds(context.Background(), "fakeCamera", map[string]interface{}{})
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no pointcloud")

	// successful, creates one object with some points in it
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		cloud := pc.New()
		err = cloud.Set(pc.NewVector(0, 0, 5), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(0, 100, 6), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(50, 0, 8), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(50, 100, 4), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(15, 15, 3), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		err = cloud.Set(pc.NewVector(16, 14, 10), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return cloud, nil
	}
	objects, err := seg.GetObjectPointClouds(context.Background(), "fakeCamera", map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(objects), test.ShouldEqual, 1)
	test.That(t, objects[0].Size(), test.ShouldEqual, 2)
	// does  implement detector
	dets, err := seg.Detections(context.Background(), nil, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(dets), test.ShouldEqual, 1)
	// does not implement classifier
	_, err = seg.Classifications(context.Background(), nil, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")
}
