//go:build !no_media

package segmentation

import (
	"context"
	"image"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
)

var (
	gripperComboParamsPath = utils.ResolveFile("vision/segmentation/data/gripper_combo_parameters.json")
	intelJSONPath          = utils.ResolveFile("vision/segmentation/data/intel.json")
	intel515ParamsPath     = utils.ResolveFile("vision/segmentation/data/intel515_parameters.json")
)

// Test finding the planes in an image with depth.
func TestPlaneSegmentImageAndDepthMap(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/planes/color", "*.png", "segmentation/planes/depth")
	logger := golog.NewTestLogger(t)
	config, err := config.Read(context.Background(), intelJSONPath, logger)
	test.That(t, err, test.ShouldBeNil)

	c := config.FindComponent("front")
	test.That(t, c, test.ShouldNotBeNil)

	aligner, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &segmentTestHelper{c.Attributes, aligner})
	test.That(t, err, test.ShouldBeNil)
}

type segmentTestHelper struct {
	attrs        utils.AttributeMap
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

func (h *segmentTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	fixedColor, fixedDepth, err := h.cameraParams.AlignColorAndDepthImage(im, dm)
	test.That(t, err, test.ShouldBeNil)
	fixedDepth, err = rimage.PreprocessDepthMap(fixedDepth, fixedColor)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(rimage.Overlay(fixedColor, fixedDepth), "overlay")

	pCtx.GotDebugImage(fixedDepth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	cloud, err := h.cameraParams.RGBDToPointCloud(fixedColor, fixedDepth)
	test.That(t, err, test.ShouldBeNil)

	// create an image where all the planes in the point cloud are color-coded
	planeSegCloud := NewPointCloudPlaneSegmentation(cloud, 50, 150000)      // feed the parameters for the plane segmentation
	planes, nonPlane, err := planeSegCloud.FindPlanes(context.Background()) // returns slice of planes and point cloud of non plane points
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planes), test.ShouldBeGreaterThan, 0)
	segments := make([]pc.PointCloud, 0, len(planes)+1) // make a slice for all plane-pointclouds and the non-plane pointcloud
	segments = append(segments, nonPlane)               // non-plane point cloud gets added first
	for _, plane := range planes {
		test.That(t, plane, test.ShouldNotBeNil)
		planeCloud, err := plane.PointCloud()
		test.That(t, err, test.ShouldBeNil)
		segments = append(segments, planeCloud) // append the point clouds of each plane to the slice
	}
	segImage, err := PointCloudSegmentsToMask(h.cameraParams.ColorCamera, segments) // color-coded image of all the planes
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(segImage, "from-pointcloud")

	// Informational histograms for voxel grid creation. This is useful for determining which lambda
	// value to choose for the voxel grid plane segmentation.
	voxelSize := 20.
	lam := 8.0
	vg := pc.NewVoxelGridFromPointCloud(cloud, voxelSize, lam)
	histPt, err := vg.VoxelHistogram(h.cameraParams.ColorCamera.Width, h.cameraParams.ColorCamera.Height, "points")
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(histPt, "voxel-histograms")
	histWt, err := vg.VoxelHistogram(h.cameraParams.ColorCamera.Width, h.cameraParams.ColorCamera.Height, "weights")
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(histWt, "weight-histograms")
	histRes, err := vg.VoxelHistogram(h.cameraParams.ColorCamera.Width, h.cameraParams.ColorCamera.Height, "residuals")
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(histRes, "residual-histograms")

	// voxel grid plane segmentation -- do the same thing as above but using the voxel grid method to get the planes.
	voxelConfig := VoxelGridPlaneConfig{
		WeightThresh:   0.9,
		AngleThresh:    80,
		CosineThresh:   0.30,
		DistanceThresh: voxelSize * 0.5,
	}
	planeSegVoxel := NewVoxelGridPlaneSegmentation(vg, voxelConfig)
	planesVox, nonPlaneVox, err := planeSegVoxel.FindPlanes(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planesVox), test.ShouldBeGreaterThan, 0)
	t.Logf("number of planes: %d", len(planesVox))

	voxSegments := make([]pc.PointCloud, 0, len(planes)+1)
	voxSegments = append(voxSegments, nonPlaneVox)
	for _, plane := range planesVox {
		planeCloud, err := plane.PointCloud()
		test.That(t, err, test.ShouldBeNil)
		voxSegments = append(voxSegments, planeCloud)
	}
	voxSegImage, err := PointCloudSegmentsToMask(h.cameraParams.ColorCamera, voxSegments)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(voxSegImage, "from-segmented-pointcloud")

	return nil
}

// testing out gripper plane segmentation.
func TestGripperPlaneSegmentation(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper/color", "*.png", "segmentation/gripper/depth")
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(gripperComboParamsPath)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperPlaneTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}

type gripperPlaneTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

func (h *gripperPlaneTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth")

	// Pre-process the depth map to smooth the noise out and fill holes
	dm, err = rimage.PreprocessDepthMap(dm, im)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth-filled")

	// Get the point cloud
	cloud, err := h.cameraParams.RGBDToPointCloud(im, dm)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "gripper-pointcloud")

	// voxel grid plane segmentation
	voxelConfig := VoxelGridPlaneConfig{
		WeightThresh:   0.9,
		AngleThresh:    30,
		CosineThresh:   0.1,
		DistanceThresh: 0.1,
	}
	vg := pc.NewVoxelGridFromPointCloud(cloud, 8.0, 0.1)
	planeSegVoxel := NewVoxelGridPlaneSegmentation(vg, voxelConfig)
	planesVox, _, err := planeSegVoxel.FindPlanes(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planesVox), test.ShouldBeGreaterThan, 0)
	t.Logf("number of planes: %d", len(planesVox))

	// point cloud plane segmentation
	planeSeg := NewPointCloudPlaneSegmentation(cloud, 10, 15000)
	planes, nonPlane, err := planeSeg.FindPlanes(context.Background())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planes), test.ShouldBeGreaterThan, 0)

	// find the planes, and only keep points above the biggest found plane
	above, _, err := SplitPointCloudByPlane(nonPlane, planes[0])
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(above, "gripper-above-pointcloud")
	heightLimit, err := ThresholdPointCloudByPlane(above, planes[0], 100.0)
	test.That(t, err, test.ShouldBeNil)

	// color the segmentation
	segments, err := segmentPointCloudObjects(heightLimit, 10.0, 5)
	test.That(t, err, test.ShouldBeNil)
	coloredSegments, err := pc.MergePointCloudsWithColor(segments)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(coloredSegments, "gripper-segments-pointcloud")

	return nil
}
