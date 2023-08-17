package obstaclesdepth

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
	d := rimage.NewEmptyDepthMap(50, 50)
	for i := 0; i < 40; i++ {
		for j := 5; j < 45; j++ {
			d.Set(i, j, rimage.Depth(400))
		}
	}
	return d, nil, nil
}

func (r testReader) Close(ctx context.Context) error {
	return nil
}

func TestObstacleDepth(t *testing.T) {
	no := false
	noIntrinsicsCfg := ObsDepthConfig{
		Hmin:           defaultHmin,
		Hmax:           defaultHmax,
		ThetaMax:       defaultThetamax,
		ReturnPCDs:     false,
		WithGeometries: &no,
	}
	someIntrinsics := transform.PinholeCameraIntrinsics{Fx: 604.5, Fy: 609.6, Ppx: 324.6, Ppy: 238.9, Width: 640, Height: 480}
	withIntrinsicsCfg := ObsDepthConfig{
		Hmin:       defaultHmin,
		Hmax:       defaultHmax,
		ThetaMax:   defaultThetamax,
		ReturnPCDs: true,
	}

	ctx := context.Background()
	testLogger := golog.NewLogger("test")
	r := &inject.Robot{ResourceNamesFunc: func() []resource.Name {
		return []resource.Name{camera.Named("testCam"), camera.Named("noIntrinsicsCam")}
	}}
	tr := testReader{}
	syst := transform.PinholeCameraModel{&someIntrinsics, nil}
	myCamSrcIntrinsics, err := camera.NewVideoSourceFromReader(ctx, tr, &syst, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myCamSrcIntrinsics, test.ShouldNotBeNil)
	myCamSrcNoIntrinsics, err := camera.NewVideoSourceFromReader(ctx, tr, nil, camera.DepthStream)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, myCamSrcNoIntrinsics, test.ShouldNotBeNil)
	myIntrinsicsCam := camera.FromVideoSource(resource.Name{Name: "testCam"}, myCamSrcIntrinsics)
	noIntrinsicsCam := camera.FromVideoSource(resource.Name{Name: "noIntrinsicsCam"}, myCamSrcNoIntrinsics)
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "testCam":
			return myIntrinsicsCam, nil
		case "noIntrinsicsCam":
			return noIntrinsicsCam, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}
	name := vision.Named("test")
	srv, err := registerObstaclesDepth(ctx, name, &noIntrinsicsCfg, r, testLogger)
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

	t.Run("no intrinsics version", func(t *testing.T) {
		// Test that it is a segmenter
		obs, err := srv.GetObjectPointClouds(ctx, "noIntrinsicsCam", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, obs, test.ShouldNotBeNil)
		test.That(t, len(obs), test.ShouldEqual, 1)
		test.That(t, obs[0].PointCloud, test.ShouldBeNil)
		poseShouldBe := spatialmath.NewPose(r3.Vector{0, 0, 400}, nil)
		test.That(t, obs[0].Geometry.Pose(), test.ShouldResemble, poseShouldBe)
	})
	t.Run("intrinsics version", func(t *testing.T) {
		// Now with intrinsics (and pointclouds)!
		srv2, err := registerObstaclesDepth(ctx, name, &withIntrinsicsCfg, r, testLogger)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, srv2, test.ShouldNotBeNil)
		obs, err := srv2.GetObjectPointClouds(ctx, "testCam", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, obs, test.ShouldNotBeNil)
		test.That(t, len(obs), test.ShouldEqual, defaultK)
		for _, o := range obs {
			test.That(t, o.PointCloud, test.ShouldNotBeNil)
			test.That(t, o.Geometry, test.ShouldNotBeNil)
		}
	})
}

func BenchmarkObstacleDepthIntrinsics(b *testing.B) {
	someIntrinsics := transform.PinholeCameraIntrinsics{Fx: 604.5, Fy: 609.6, Ppx: 324.6, Ppy: 238.9, Width: 640, Height: 480}
	withIntrinsicsCfg := ObsDepthConfig{
		Hmin:       defaultHmin,
		Hmax:       defaultHmax,
		ThetaMax:   defaultThetamax,
		ReturnPCDs: true,
	}

	ctx := context.Background()
	testLogger := golog.NewLogger("test")
	r := &inject.Robot{ResourceNamesFunc: func() []resource.Name {
		return []resource.Name{camera.Named("testCam")}
	}}
	tr := testReader{}
	syst := transform.PinholeCameraModel{&someIntrinsics, nil}
	myCamSrc, _ := camera.NewVideoSourceFromReader(ctx, tr, &syst, camera.DepthStream)
	myCam := camera.FromVideoSource(resource.Name{Name: "testCam"}, myCamSrc)
	r.ResourceByNameFunc = func(n resource.Name) (resource.Resource, error) {
		switch n.Name {
		case "testCam":
			return myCam, nil
		default:
			return nil, resource.NewNotFoundError(n)
		}
	}
	name := vision.Named("test")
	srv, _ := registerObstaclesDepth(ctx, name, &withIntrinsicsCfg, r, testLogger)

	for i := 0; i < b.N; i++ {
		srv.GetObjectPointClouds(ctx, "testCam", nil)
	}
}
