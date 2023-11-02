package segmentation_test

import (
	"context"
	"image"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision/segmentation"
)

var (
	gripperComboParamsPath = utils.ResolveFile("vision/segmentation/data/gripper_combo_parameters.json")
	intel515ParamsPath     = utils.ResolveFile("vision/segmentation/data/intel515_parameters.json")
)

// Test finding the objects in an aligned intel image.
type segmentObjectTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

// Process creates a segmentation using raw PointClouds and then VoxelGrids.
func (h *segmentObjectTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger logging.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(context.Background(), img2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	pCtx.GotDebugImage(rimage.Overlay(im, dm), "overlay")

	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	cloud, err := h.cameraParams.RGBDToPointCloud(im, dm)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "intel-full-pointcloud")
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return cloud, nil
	}

	objConfig := utils.AttributeMap{
		"min_points_in_plane":   50000,
		"min_points_in_segment": 500,
		"clustering_radius_mm":  10.0,
		"mean_k_filtering":      50,
	}

	// Do object segmentation with point clouds
	segmenter, err := segmentation.NewRadiusClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	segments, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)

	objectClouds := []pc.PointCloud{}
	for _, seg := range segments {
		objectClouds = append(objectClouds, seg.PointCloud)
	}
	coloredSegments, err := pc.MergePointCloudsWithColor(objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(coloredSegments, "intel-segments-pointcloud")

	segImage, err := segmentation.PointCloudSegmentsToMask(h.cameraParams.ColorCamera, objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(segImage, "segmented-pointcloud-image-with-depth")

	return nil
}

func TestObjectSegmentationAlignedIntel(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel/color", "*.png", "segmentation/aligned_intel/depth")
	aligner, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(intel515ParamsPath)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &segmentObjectTestHelper{aligner})
	test.That(t, err, test.ShouldBeNil)
}

// Test finding objects in images from the gripper camera.
type gripperSegmentTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

func (h *gripperSegmentTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img, img2 image.Image,
	logger logging.Logger,
) error {
	t.Helper()
	var err error
	im := rimage.ConvertImage(img)
	dm, _ := rimage.ConvertImageToDepthMap(context.Background(), img2)
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
	injectCamera := &inject.Camera{}
	injectCamera.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return cloud, nil
	}

	// Do object segmentation with point clouds
	objConfig := utils.AttributeMap{
		"min_points_in_plane":   15000,
		"min_points_in_segment": 100,
		"clustering_radius_mm":  10.0,
		"mean_k_filtering":      10,
	}

	// Do object segmentation with point clouds
	segmenter, err := segmentation.NewRadiusClustering(objConfig)
	test.That(t, err, test.ShouldBeNil)
	segments, err := segmenter(context.Background(), injectCamera)
	test.That(t, err, test.ShouldBeNil)

	objectClouds := []pc.PointCloud{}
	for _, seg := range segments {
		objectClouds = append(objectClouds, seg.PointCloud)
	}

	coloredSegments, err := pc.MergePointCloudsWithColor(objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(coloredSegments, "gripper-segments-pointcloud")

	segImage, err := segmentation.PointCloudSegmentsToMask(h.cameraParams.ColorCamera, objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(segImage, "gripper-segmented-pointcloud-image-with-depth")

	return nil
}

func TestGripperObjectSegmentation(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper/color", "*.png", "segmentation/gripper/depth")
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(gripperComboParamsPath)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}
