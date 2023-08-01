package obstacledepth

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

// testReader creates and serves a fake depth image for testing.
type testReader struct{}

func (r testReader) Read(ctx context.Context) (image.Image, func(), error) {
	d := rimage.NewEmptyDepthMap(640, 480)
	for i := 200; i < 300; i++ {
		for j := 250; j < 350; j++ {
			d.Set(i, j, rimage.Depth(400))
		}
	}
	return d, nil, nil
}

func (r testReader) Close(ctx context.Context) error {
	return nil
}

func TestObstacleDist(t *testing.T) {
	noIntrinsicsCfg := ObstaclesDepthConfig{
		K:          10,
		Hmin:       defaultHmin,
		Hmax:       defaultHmax,
		ThetaMax:   defaultThetamax,
		ReturnPCDs: false,
	}
	someIntrinsics := transform.PinholeCameraIntrinsics{Fx: 604.5, Fy: 609.6, Ppx: 324.6, Ppy: 238.9, Width: 640, Height: 480}
	withIntrinsicsCfg := ObstaclesDepthConfig{
		K:          12,
		Hmin:       defaultHmin,
		Hmax:       defaultHmax,
		ThetaMax:   defaultThetamax,
		ReturnPCDs: true,
		intrinsics: &someIntrinsics,
	}

	ctx := context.Background()
	r := &inject.Robot{LoggerFunc: func() golog.Logger {
		return golog.NewLogger("test")
	}, ResourceNamesFunc: func() []resource.Name {
		return []resource.Name{camera.Named("testCam")}
	}}
	tr := testReader{}
	myCamSrc, err := camera.NewVideoSourceFromReader(ctx, tr, nil, camera.DepthStream)
	myCam := camera.FromVideoSource(resource.Name{Name: "testCam"}, myCamSrc)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myCamSrc, test.ShouldNotBeNil)
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "testCam":
			return myCam, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}
	name := vision.Named("test")
	srv, err := registerObstacleDepth(ctx, name, &noIntrinsicsCfg, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv.Name(), test.ShouldResemble, name)

	// Not a detector or classifier
	img, err := rimage.NewImageFromFile(artifact.MustPath("vision/objectdetection/detection_test.jpg"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img, test.ShouldNotBeNil)
	_, err = srv.Detections(ctx, img, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")
	_, err = srv.Classifications(ctx, img, 1, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement")

	// Test that it is a segmenter
	obs, err := srv.GetObjectPointClouds(ctx, "testCam", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, obs, test.ShouldNotBeNil)
	test.That(t, len(obs), test.ShouldEqual, 1)
	test.That(t, obs[0].PointCloud, test.ShouldBeNil)
	poseShouldBe := spatialmath.NewPose(r3.Vector{0, 0, 400}, nil)
	test.That(t, obs[0].Geometry.Pose(), test.ShouldResemble, poseShouldBe)

	// Now with intrinsics (and pointclouds)!
	srv2, err := registerObstacleDepth(ctx, name, &withIntrinsicsCfg, r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, srv2, test.ShouldNotBeNil)
	obs, err = srv2.GetObjectPointClouds(ctx, "testCam", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, obs, test.ShouldNotBeNil)
	test.That(t, len(obs), test.ShouldEqual, withIntrinsicsCfg.K)
	for _, o := range obs {
		test.That(t, o.PointCloud, test.ShouldNotBeNil)
		test.That(t, o.Geometry, test.ShouldNotBeNil)
	}
}
