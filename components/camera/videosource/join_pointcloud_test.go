package videosource

import (
	"context"
	"image"
	"image/color"
	"os"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
		pc1 := pointcloud.NewWithPrealloc(1)
		err := pc1.Set(pointcloud.NewVector(1, 0, 0), pointcloud.NewColoredData(color.NRGBA{255, 0, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return pc1, nil
	}
	cam1.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	cam1.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	cam2 := &inject.Camera{}
	cam2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc2 := pointcloud.NewWithPrealloc(1)
		err := pc2.Set(pointcloud.NewVector(0, 1, 0), pointcloud.NewColoredData(color.NRGBA{0, 255, 0, 255}))
		test.That(t, err, test.ShouldBeNil)
		return pc2, nil
	}
	cam2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	cam2.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	cam3 := &inject.Camera{}
	cam3.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc3 := pointcloud.NewWithPrealloc(1)
		err := pc3.Set(pointcloud.NewVector(0, 0, 1), pointcloud.NewColoredData(color.NRGBA{0, 0, 255, 255}))
		test.That(t, err, test.ShouldBeNil)
		return pc3, nil
	}
	cam3.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	cam3.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	base1 := &inject.Base{}

	r := &inject.Robot{}
	fsParts := framesystemparts.Parts{
		{
			Name:        "base1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: r3.Vector{0, 0, 0}},
		},
		{
			Name:        "cam1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: r3.Vector{100, 0, 0}},
		},
		{
			Name:        "cam2",
			FrameConfig: &config.Frame{Parent: "cam1", Translation: r3.Vector{0, 0, 100}},
		},
		{
			Name:        "cam3",
			FrameConfig: &config.Frame{Parent: "cam2", Translation: r3.Vector{0, 100, 0}},
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

func TestJoinPointCloudNaive(t *testing.T) {
	r := makeFakeRobot(t)
	defer utils.TryClose(context.Background(), r)
	// PoV from base1
	attrs := &JoinAttrs{
		Debug:         true,
		SourceCameras: []string{"cam1", "cam2", "cam3"},
		TargetFrame:   "base1",
		MergeMethod:   "naive",
	}
	joinedCam, err := newJoinPointCloudSource(context.Background(), r, golog.NewTestLogger(t), attrs)
	test.That(t, err, test.ShouldBeNil)
	pc, err := joinedCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
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

	img, _, err := camera.ReadImage(context.Background(), joinedCam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1, 100))
	test.That(t, joinedCam.Close(context.Background()), test.ShouldBeNil)

	// PoV from cam1
	attrs2 := &JoinAttrs{
		Debug:         true,
		SourceCameras: []string{"cam1", "cam2", "cam3"},
		TargetFrame:   "cam1",
		MergeMethod:   "naive",
	}
	joinedCam2, err := newJoinPointCloudSource(context.Background(), r, utils.Logger, attrs2)
	test.That(t, err, test.ShouldBeNil)
	pc, err = joinedCam2.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
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

	img, _, err = camera.ReadImage(context.Background(), joinedCam2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, img.Bounds(), test.ShouldResemble, image.Rect(0, 0, 1, 100))
	test.That(t, joinedCam2.Close(context.Background()), test.ShouldBeNil)
}

func makePointCloudFromArtifact(t *testing.T, artifactPath string, numPoints int) (pointcloud.PointCloud, error) {
	t.Helper()
	pcdFile, err := os.Open(artifact.MustPath(artifactPath))
	if err != nil {
		return nil, err
	}
	pc, err := pointcloud.ReadPCD(pcdFile)
	if err != nil {
		return nil, err
	}

	if numPoints == 0 {
		return pc, nil
	}

	shortenedPC := pointcloud.NewWithPrealloc(numPoints)

	counter := numPoints
	pc.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		if counter > 0 {
			err = shortenedPC.Set(p, d)
			counter--
		}
		return err == nil
	})
	if err != nil {
		return nil, err
	}

	return shortenedPC, nil
}

func makeFakeRobotICP(t *testing.T) (robot.Robot, error) {
	// Makes a fake robot with a fake frame system and multiple cameras for testing.
	// Cam 1: Read from a test PCD file. A smaller sample of points.
	// Cam 2: A direct transformation applied to Cam 1.
	// This is useful for basic checking of the ICP algorithm, as it should converge immediately.
	// Cam 3: Read from a test PCD file. Representative of a real pointcloud captured in tandem with Cam 4.
	// Cam 4: Read from a test PCD file. Captured in a real environment with a known rough offset from Cam 3.

	// Cam 1 and 2 Are programatically set to have a difference of 100 in the Z direction.
	// Cam 3 and 4 Sensors are approximately 33 cm apart with an unknown slight rotation.
	t.Helper()
	logger := golog.NewTestLogger(t)
	cam1 := &inject.Camera{}
	startPC, err := makePointCloudFromArtifact(t, "pointcloud/test.pcd", 100)
	if err != nil {
		t.Fatal(err)
	}
	cam1.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return startPC, nil
	}
	cam1.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	cam1.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}
	cam2 := &inject.Camera{}
	transformedPC := pointcloud.NewWithPrealloc(100)
	transformPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 100})
	counter := 100
	startPC.Iterate(0, 0, func(p r3.Vector, d pointcloud.Data) bool {
		if counter > 0 {
			pointPose := spatialmath.NewPoseFromPoint(p)
			transPoint := spatialmath.Compose(transformPose, pointPose)
			err = transformedPC.Set(transPoint.Point(), d)
			if err != nil {
				return false
			}
			counter--
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	cam2.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return transformedPC, nil
	}
	cam2.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}
	cam2.ProjectorFunc = func(ctx context.Context) (transform.Projector, error) {
		return nil, transform.NewNoIntrinsicsError("")
	}

	cam3 := &inject.Camera{}
	pc3, err := makePointCloudFromArtifact(t, "pointcloud/bun000.pcd", 0)
	if err != nil {
		t.Fatal(err)
	}

	cam3.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pc3, nil
	}
	cam3.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	cam4 := &inject.Camera{}
	pc4, err := makePointCloudFromArtifact(t, "pointcloud/bun045.pcd", 0)
	if err != nil {
		t.Fatal(err)
	}

	cam4.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pc4, nil
	}
	cam4.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	cam5 := &inject.Camera{}
	pc5, err := makePointCloudFromArtifact(t, "pointcloud/bun090.pcd", 0)
	if err != nil {
		t.Fatal(err)
	}

	cam5.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		return pc5, nil
	}
	cam5.PropertiesFunc = func(ctx context.Context) (camera.Properties, error) {
		return camera.Properties{}, nil
	}

	base1 := &inject.Base{}

	r := &inject.Robot{}
	fsParts := framesystemparts.Parts{
		{
			Name:        "base1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: r3.Vector{0, 0, 0}},
		},
		{
			Name:        "cam1",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: r3.Vector{0, 0, 0}},
		},
		{
			Name:        "cam2",
			FrameConfig: &config.Frame{Parent: "cam1", Translation: r3.Vector{0, 0, -100}},
		},
		{
			Name:        "cam3",
			FrameConfig: &config.Frame{Parent: referenceframe.World, Translation: r3.Vector{0, 0, 0}},
		},
		{
			Name: "cam4",
			FrameConfig: &config.Frame{
				Parent: "cam3", Translation: r3.Vector{-60, 0, -10},
				Orientation: &spatialmath.EulerAngles{Roll: 0, Pitch: 0.6, Yaw: 0},
			},
		},
		{
			Name: "cam5",
			FrameConfig: &config.Frame{
				Parent: "cam4", Translation: r3.Vector{-60, 0, 10},
				Orientation: &spatialmath.EulerAngles{Roll: 0, Pitch: 0.6, Yaw: -0.3},
			},
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
		return []resource.Name{
			camera.Named("cam1"), camera.Named("cam2"), camera.Named("cam3"),
			camera.Named("cam4"), camera.Named("cam5"), base.Named("base1"),
		}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "cam1":
			return cam1, nil
		case "cam2":
			return cam2, nil
		case "cam3":
			return cam3, nil
		case "cam4":
			return cam4, nil
		case "cam5":
			return cam5, nil
		case "base1":
			return base1, nil
		default:
			return nil, rdkutils.NewResourceNotFoundError(n)
		}
	}
	return r, nil
}

func TestFixedPointCloudICP(t *testing.T) {
	ctx := context.Background()
	r, err := makeFakeRobotICP(t)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(500 * time.Millisecond)
	// PoV from base1
	attrs := &JoinAttrs{
		Debug:         true,
		SourceCameras: []string{"cam1", "cam2"},
		TargetFrame:   "base1",
		MergeMethod:   "icp",
		Closeness:     0.01,
	}
	joinedCam, err := newJoinPointCloudSource(ctx, r, utils.Logger, attrs)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), joinedCam)
	pc, err := joinedCam.NextPointCloud(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 100)
}

func TestTwinPointCloudICP(t *testing.T) {
	t.Skip("Test is too large for now.")
	r, err := makeFakeRobotICP(t)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(500 * time.Millisecond)

	attrs := &JoinAttrs{
		Debug:         true,
		SourceCameras: []string{"cam3", "cam4"},
		TargetFrame:   "cam3",
		MergeMethod:   "icp",
	}
	joinedCam, err := newJoinPointCloudSource(context.Background(), r, utils.Logger, attrs)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), joinedCam)
	pc, err := joinedCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)
	filename := "test_twin_" + time.Now().Format(time.RFC3339) + "*.pcd"
	file, err := os.CreateTemp("/tmp", filename)
	pointcloud.ToPCD(pc, file, pointcloud.PCDBinary)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
}

func TestMultiPointCloudICP(t *testing.T) {
	t.Skip("Test is too large for now.")
	r, err := makeFakeRobotICP(t)
	test.That(t, err, test.ShouldBeNil)
	time.Sleep(500 * time.Millisecond)

	attrs := &JoinAttrs{
		Debug:         true,
		SourceCameras: []string{"cam3", "cam4", "cam5"},
		TargetFrame:   "cam3",
		MergeMethod:   "icp",
	}
	joinedCam, err := newJoinPointCloudSource(context.Background(), r, utils.Logger, attrs)
	test.That(t, err, test.ShouldBeNil)
	defer utils.TryClose(context.Background(), joinedCam)
	pc, err := joinedCam.NextPointCloud(context.Background())
	test.That(t, err, test.ShouldBeNil)

	filename := "test_multi_" + time.Now().Format(time.RFC3339) + "*.pcd"
	file, err := os.CreateTemp("/tmp", filename)
	pointcloud.ToPCD(pc, file, pointcloud.PCDBinary)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc, test.ShouldNotBeNil)
}
