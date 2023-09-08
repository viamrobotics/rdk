//go:build !notc

package segmentation

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/depthadapter"
	"go.viam.com/rdk/rimage/transform"
)

func init() {
	sortPositions = true
}

func TestPlaneConfig(t *testing.T) {
	cfg := VoxelGridPlaneConfig{}
	// invalid weight threshold
	cfg.WeightThresh = -2.
	err := cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "weight_threshold cannot be less than 0")
	// invalid angle threshold
	cfg.WeightThresh = 1.
	cfg.AngleThresh = 1000.
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "angle_threshold must be in degrees, between -360 and 360")
	// invalid cosine threshold
	cfg.AngleThresh = 30.
	cfg.CosineThresh = 2.
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "cosine_threshold must be between -1 and 1")
	// invalid distance threshold
	cfg.CosineThresh = 0.2
	cfg.DistanceThresh = -5
	err = cfg.CheckValid()
	test.That(t, err.Error(), test.ShouldContainSubstring, "distance_threshold cannot be less than 0")
	// valid
	cfg.DistanceThresh = 5
	err = cfg.CheckValid()
	test.That(t, err, test.ShouldBeNil)
}

func TestSegmentPlaneWRTGround(t *testing.T) {
	// get depth map
	d, err := rimage.NewDepthMapFromFile(
		context.Background(),
		artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.png"))
	test.That(t, err, test.ShouldBeNil)

	// Pixel to Meter
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)
	depthIntrinsics := &sensorParams.DepthCamera
	cloud := depthadapter.ToPointCloud(d, depthIntrinsics)
	test.That(t, err, test.ShouldBeNil)
	// Segment Plane
	nIter := 3000
	groundNormVec := r3.Vector{0, 1, 0}
	angleThresh := 30.0
	plane, _, err := SegmentPlaneWRTGround(context.Background(), cloud, nIter, angleThresh, 0.5, groundNormVec)
	eq := plane.Equation()
	test.That(t, err, test.ShouldBeNil)

	p1 := r3.Vector{-eq[3] / eq[0], 0, 0}
	p2 := r3.Vector{0, -eq[3] / eq[1], 0}
	p3 := r3.Vector{0, 0, -eq[3] / eq[2]}

	v1 := p2.Sub(p1).Normalize()
	v2 := p3.Sub(p1).Normalize()

	planeNormVec := v1.Cross(v2)
	planeNormVec = planeNormVec.Normalize()
	test.That(t, math.Acos(planeNormVec.Dot(groundNormVec)), test.ShouldBeLessThanOrEqualTo, angleThresh*math.Pi/180)
}

func TestSegmentPlane(t *testing.T) {
	// Intel Sensor Extrinsic data from manufacturer
	// Intel sensor depth 1024x768 to  RGB 1280x720
	// Translation Vector : [-0.000828434,0.0139185,-0.0033418]
	// Rotation Matrix    : [0.999958,-0.00838489,0.00378392]
	//                   : [0.00824708,0.999351,0.0350734]
	//                   : [-0.00407554,-0.0350407,0.999378]
	// Intel sensor RGB 1280x720 to depth 1024x768
	// Translation Vector : [0.000699992,-0.0140336,0.00285468]
	// Rotation Matrix    : [0.999958,0.00824708,-0.00407554]
	//                   : [-0.00838489,0.999351,-0.0350407]
	//                   : [0.00378392,0.0350734,0.999378]
	// Intel sensor depth 1024x768 intrinsics
	// Principal Point         : 542.078, 398.016
	// Focal Length            : 734.938, 735.516
	// get depth map
	d, err := rimage.NewDepthMapFromFile(
		context.Background(),
		artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.png"))
	test.That(t, err, test.ShouldBeNil)

	// Pixel to Meter
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)
	depthIntrinsics := &sensorParams.DepthCamera
	cloud := depthadapter.ToPointCloud(d, depthIntrinsics)
	test.That(t, err, test.ShouldBeNil)
	// Segment Plane
	nIter := 3000
	plane, _, err := SegmentPlane(context.Background(), cloud, nIter, 0.5)
	eq := plane.Equation()
	test.That(t, err, test.ShouldBeNil)
	// assign gt plane equation - obtained from open3d library with the same parameters
	gtPlaneEquation := make([]float64, 4)
	// gtPlaneEquation =  0.02x + 1.00y + 0.09z + -1.12 = 0, obtained from Open3D
	gtPlaneEquation[0] = 0.02
	gtPlaneEquation[1] = 1.0
	gtPlaneEquation[2] = 0.09
	gtPlaneEquation[3] = -1.12

	dot := eq[0]*gtPlaneEquation[0] + eq[1]*gtPlaneEquation[1] + eq[2]*gtPlaneEquation[2]
	tol := 0.75
	test.That(t, math.Abs(dot), test.ShouldBeGreaterThanOrEqualTo, tol)
}

func TestDepthMapToPointCloud(t *testing.T) {
	d, err := rimage.NewDepthMapFromFile(
		context.Background(),
		artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.png"))
	test.That(t, err, test.ShouldBeNil)
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)
	depthIntrinsics := &sensorParams.DepthCamera
	pc := depthadapter.ToPointCloud(d, depthIntrinsics)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 456370)
}

func TestProjectPlane3dPointsToRGBPlane(t *testing.T) {
	t.Parallel()
	rgb, err := rimage.NewImageFromFile(artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036_color.png"))
	test.That(t, err, test.ShouldBeNil)
	d, err := rimage.NewDepthMapFromFile(
		context.Background(),
		artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.png"))
	test.That(t, err, test.ShouldBeNil)
	h, w := rgb.Height(), rgb.Width()

	// Get 3D Points
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)
	pts := depthadapter.ToPointCloud(d, &sensorParams.DepthCamera)
	test.That(t, err, test.ShouldBeNil)
	// Get rigid body transform between Depth and RGB sensor
	// Apply RBT
	transformedPoints, err := sensorParams.ApplyRigidBodyTransform(pts)
	test.That(t, err, test.ShouldBeNil)
	// Re-project 3D Points in RGB Plane
	pixel2meter := 0.001
	coordinatesRGB, err := transform.ProjectPointCloudToRGBPlane(transformedPoints, h, w, sensorParams.ColorCamera, pixel2meter)
	test.That(t, err, test.ShouldBeNil)
	// fill image
	upLeft := image.Point{0, 0}
	lowRight := image.Point{w, h}

	img := image.NewGray16(image.Rectangle{upLeft, lowRight})
	coordinatesRGB.Iterate(0, 0, func(pt r3.Vector, d pc.Data) bool {
		if pt.Z > -1.0 {
			img.Set(int(pt.X), int(pt.Y), color.Gray16{uint16(pt.Z / pixel2meter)})
		}
		return true
	})

	maxPt := img.Bounds().Max
	test.That(t, maxPt.X, test.ShouldEqual, rgb.Width())
	test.That(t, maxPt.Y, test.ShouldEqual, rgb.Height())
}

func BenchmarkPlaneSegmentPointCloud(b *testing.B) {
	d, err := rimage.NewDepthMapFromFile(
		context.Background(),
		artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.png"))
	test.That(b, err, test.ShouldBeNil)

	// Pixel to Meter
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(b, err, test.ShouldBeNil)
	depthIntrinsics := &sensorParams.DepthCamera
	test.That(b, err, test.ShouldBeNil)
	pts := depthadapter.ToPointCloud(d, depthIntrinsics)
	test.That(b, err, test.ShouldBeNil)
	for i := 0; i < b.N; i++ {
		// Segment Plane
		_, _, err := SegmentPlane(context.Background(), pts, 2500, 0.0025)
		test.That(b, err, test.ShouldBeNil)
	}
}

func TestPointCloudSplit(t *testing.T) {
	// make a simple point cloud
	cloud := pc.New()
	var err error
	err = cloud.Set(pc.NewVector(1, 1, 1), pc.NewColoredData(color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewVector(1, 0, -1), pc.NewColoredData(color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewVector(-1, -2, -1), pc.NewColoredData(color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewVector(0, 0, 0), pc.NewColoredData(color.NRGBA{0, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	// 2-2 split map
	map1 := map[r3.Vector]bool{
		{1, 1, 1}: true,
		{0, 0, 0}: true,
	}
	mapCloud, nonMapCloud, err := pointCloudSplit(cloud, map1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mapCloud.Size(), test.ShouldEqual, 2)
	test.That(t, nonMapCloud.Size(), test.ShouldEqual, 2)
	// map of all points
	map2 := map[r3.Vector]bool{
		{1, 1, 1}:    true,
		{0, 0, 0}:    true,
		{-1, -2, -1}: true,
		{1, 0, -1}:   true,
	}
	mapCloud, nonMapCloud, err = pointCloudSplit(cloud, map2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mapCloud.Size(), test.ShouldEqual, 4)
	test.That(t, nonMapCloud.Size(), test.ShouldEqual, 0)
	// empty map
	map3 := map[r3.Vector]bool{}
	mapCloud, nonMapCloud, err = pointCloudSplit(cloud, map3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mapCloud.Size(), test.ShouldEqual, 0)
	test.That(t, nonMapCloud.Size(), test.ShouldEqual, 4)
	// map with invalid points
	map4 := map[r3.Vector]bool{
		{1, 1, 1}: true,
		{0, 2, 0}: true,
	}
	mapCloud, nonMapCloud, err = pointCloudSplit(cloud, map4)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, mapCloud, test.ShouldBeNil)
	test.That(t, nonMapCloud, test.ShouldBeNil)
}
