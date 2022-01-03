package segmentation

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

func init() {
	sortPositions = true
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
	rgbd, err := rimage.ReadBothFromFile(artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	m := rgbd.Depth
	// rgb := rgbd.Color

	test.That(t, err, test.ShouldBeNil)

	// Pixel to Meter
	pixel2meter := 0.001
	depthIntrinsics, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
		"depth",
	)
	test.That(t, err, test.ShouldBeNil)
	depthMin, depthMax := rimage.Depth(100), rimage.Depth(2000)
	cloud, err := transform.DepthMapToPointCloud(m, pixel2meter, depthIntrinsics, depthMin, depthMax)
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
	rgbd, err := rimage.ReadBothFromFile(artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	m := rgbd.Depth
	// rgb := rgbd.Color

	test.That(t, err, test.ShouldBeNil)
	pixel2meter := 0.001
	depthIntrinsics, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
		"depth",
	)
	test.That(t, err, test.ShouldBeNil)
	depthMin, depthMax := rimage.Depth(0), rimage.Depth(math.MaxUint16)
	pc, err := transform.DepthMapToPointCloud(m, pixel2meter, depthIntrinsics, depthMin, depthMax)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldEqual, 456371)
}

func TestProjectPlane3dPointsToRGBPlane(t *testing.T) {
	rgbd, err := rimage.ReadBothFromFile(artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.both.gz"), false)
	test.That(t, err, test.ShouldBeNil)
	m := rgbd.Depth
	rgb := rgbd.Color
	h, w := rgb.Height(), rgb.Width()

	test.That(t, err, test.ShouldBeNil)

	// Pixel to Meter
	pixel2meter := 0.001
	// Select depth range
	// Get 3D Points
	depthIntrinsics, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
		"depth",
	)
	test.That(t, err, test.ShouldBeNil)
	depthMin, depthMax := rimage.Depth(200), rimage.Depth(2000)
	pts, err := transform.DepthMapToPointCloud(m, pixel2meter, depthIntrinsics, depthMin, depthMax)
	test.That(t, err, test.ShouldBeNil)
	// Get rigid body transform between Depth and RGB sensor
	sensorParams, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)
	// Apply RBT
	transformedPoints, err := transform.ApplyRigidBodyTransform(pts, &sensorParams.ExtrinsicD2C)
	test.That(t, err, test.ShouldBeNil)
	// Re-project 3D Points in RGB Plane
	colorIntrinsics, err :=
		transform.NewPinholeCameraIntrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"), "color")
	test.That(t, err, test.ShouldBeNil)
	coordinatesRGB, err := transform.ProjectPointCloudToRGBPlane(transformedPoints, h, w, *colorIntrinsics, pixel2meter)
	test.That(t, err, test.ShouldBeNil)
	// fill image
	upLeft := image.Point{0, 0}
	lowRight := image.Point{w, h}

	img := image.NewGray16(image.Rectangle{upLeft, lowRight})
	coordinatesRGB.Iterate(func(pt pc.Point) bool {
		if pt.Position().Z > -1.0 {
			img.Set(int(pt.Position().X), int(pt.Position().Y), color.Gray16{uint16(pt.Position().Z / pixel2meter)})
		}
		return true
	})

	maxPt := img.Bounds().Max
	test.That(t, maxPt.X, test.ShouldEqual, rgb.Width())
	test.That(t, maxPt.Y, test.ShouldEqual, rgb.Height())
}

func BenchmarkPlaneSegmentPointCloud(b *testing.B) {
	rgbd, err := rimage.ReadBothFromFile(artifact.MustPath("vision/segmentation/pointcloudsegmentation/align-test-1615172036.both.gz"), false)
	test.That(b, err, test.ShouldBeNil)
	m := rgbd.Depth
	// rgb := rgbd.Color

	// Pixel to Meter
	pixel2meter := 0.001
	depthIntrinsics, err := transform.NewPinholeCameraIntrinsicsFromJSONFile(
		utils.ResolveFile("robots/configs/intel515_parameters.json"),
		"depth",
	)
	test.That(b, err, test.ShouldBeNil)
	depthMin, depthMax := rimage.Depth(100), rimage.Depth(2000)
	pts, err := transform.DepthMapToPointCloud(m, pixel2meter, depthIntrinsics, depthMin, depthMax)
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
	err = cloud.Set(pc.NewColoredPoint(1, 1, 1, color.NRGBA{255, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewColoredPoint(1, 0, -1, color.NRGBA{0, 255, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewColoredPoint(-1, -2, -1, color.NRGBA{0, 0, 255, 255}))
	test.That(t, err, test.ShouldBeNil)
	err = cloud.Set(pc.NewColoredPoint(0, 0, 0, color.NRGBA{0, 0, 0, 255}))
	test.That(t, err, test.ShouldBeNil)
	// 2-2 split map
	map1 := map[pc.Vec3]bool{
		{1, 1, 1}: true,
		{0, 0, 0}: true,
	}
	mapCloud, nonMapCloud, err := pointCloudSplit(cloud, map1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mapCloud.Size(), test.ShouldEqual, 2)
	test.That(t, nonMapCloud.Size(), test.ShouldEqual, 2)
	// map of all points
	map2 := map[pc.Vec3]bool{
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
	map3 := map[pc.Vec3]bool{}
	mapCloud, nonMapCloud, err = pointCloudSplit(cloud, map3)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, mapCloud.Size(), test.ShouldEqual, 0)
	test.That(t, nonMapCloud.Size(), test.ShouldEqual, 4)
	// map with invalid points
	map4 := map[pc.Vec3]bool{
		{1, 1, 1}: true,
		{0, 2, 0}: true,
	}
	mapCloud, nonMapCloud, err = pointCloudSplit(cloud, map4)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, mapCloud, test.ShouldBeNil)
	test.That(t, nonMapCloud, test.ShouldBeNil)
}
