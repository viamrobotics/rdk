package imagesource

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/robot"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
)

func makeFakeRobot(t *testing.T) robot.Robot {
	t.Helper()
	logger := golog.NewTestLogger(t)
	cam1 := &inject.Camera{}
	cam1.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		return pc, pc.Set(pointcloud.NewVector(1, 0, 0), pointcloud.NewColoredData(color.NRGBA{255, 0, 0, 255}))
	}
	cam1.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	cam2 := &inject.Camera{}
	cam2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		return pc, pc.Set(pointcloud.NewVector(0, 1, 0), pointcloud.NewColoredData(color.NRGBA{0, 255, 0, 255}))
	}
	cam2.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	cam3 := &inject.Camera{}
	cam3.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		return pc, pc.Set(pointcloud.NewVector(0, 0, 1), pointcloud.NewColoredData(color.NRGBA{0, 0, 255, 255}))
	}
	cam3.GetPropertiesFunc = func(ctx context.Context) (rimage.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	base1 := &inject.Base{}

	r := &inject.Robot{}
	fsParts := framesystemparts.Parts{
		{
			Name:        "base1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: spatialmath.TranslationConfig{0, 0, 0}},
		},
		{
			Name:        "cam1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: spatialmath.TranslationConfig{100, 0, 0}},
		},
		{
			Name:        "cam2",
			FrameConfig: &config.Frame{Parent: "cam1", Translation: spatialmath.TranslationConfig{0, 0, 100}},
		},
		{
			Name:        "cam3",
			FrameConfig: &config.Frame{Parent: "cam2", Translation: spatialmath.TranslationConfig{0, 100, 0}},
		},
	}
	r.FrameSystemConfigFunc = func(
		ctx context.Context, additionalTransforms []*commonpb.Transform,
	) (framesystemparts.Parts, error) {
		return fsParts, nil
	}

	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{camera.Named("cam1"), camera.Named("cam2"), camera.Named("cam3"), base.Named("base1")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "cam1":
			return cam1, nil
		case "cam2":
			return cam2, nil
		case "cam3":
			return cam3, nil
		case "base1":
			return base1, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}
	return r
}

func TestJoinPointCloud(t *testing.T) {
	r := makeFakeRobot(t)
	// PoV from base1
	attrs := &JoinAttrs{
		AttrConfig:    &camera.AttrConfig{},
		SourceCameras: []string{"cam1", "cam2", "cam3"},
		TargetFrame:   "base1",
	}
	joinedCam, err := newJoinPointCloudSource(context.Background(), r, attrs)
	test.That(t, err, test.ShouldBeNil)
	pc, err := joinedCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 3)

	data, got := pc.At(101, 0, 0)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{255, 0, 0, 255})

	data, got = pc.At(100, 1, 100)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 255, 0, 255})

	data, got = pc.At(100, 100, 101)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 0, 255, 255})

	img, _, err := joinedCam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1, 100))

	// PoV from cam1
	attrs = &JoinAttrs{
		AttrConfig:    &camera.AttrConfig{},
		SourceCameras: []string{"cam1", "cam2", "cam3"},
		TargetFrame:   "cam1",
	}
	joinedCam, err = newJoinPointCloudSource(context.Background(), r, attrs)
	test.That(t, err, test.ShouldBeNil)
	pc, err = joinedCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 3)

	data, got = pc.At(1, 0, 0)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{255, 0, 0, 255})

	data, got = pc.At(0, 1, 100)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 255, 0, 255})

	data, got = pc.At(0, 100, 101)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, data.Color(), test.ShouldResemble, &color.NRGBA{0, 0, 255, 255})

	img, _, err = joinedCam.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1, 100))
}
