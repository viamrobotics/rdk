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

// Test finding objects in images from the gripper camera in voxel.
type gripperVoxelSegmentTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

func (h *gripperVoxelSegmentTestHelper) Process(
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

	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth")

	// Pre-process the depth map to smooth the noise out and fill holes
	dm, err = rimage.PreprocessDepthMap(dm, im)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(dm.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth-filled")

	// Get the point cloud
	cloud, err := h.cameraParams.RGBDToPointCloud(im, dm)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "gripper-pointcloud")
	cam := &inject.Camera{}
	cam.NextPointCloudFunc = func(ctx context.Context) (pc.PointCloud, error) {
		return cloud, nil
	}

	// Do voxel segmentation
	voxObjConfig := utils.AttributeMap{
		"voxel_size":            5.0,
		"lambda":                1.0,
		"min_points_in_plane":   15000,
		"min_points_in_segment": 100,
		"clustering_radius_mm":  7.5,
		"weight_threshold":      0.9,
		"angle_threshold":       30,
		"cosine_threshold":      0.1,
		"distance_threshold":    0.1,
	}

	segmenter, err := segmentation.NewRadiusClusteringFromVoxels(voxObjConfig)
	test.That(t, err, test.ShouldBeNil)
	voxSegments, err := segmenter(context.Background(), cam)
	test.That(t, err, test.ShouldBeNil)

	voxObjectClouds := []pc.PointCloud{}
	for _, seg := range voxSegments {
		voxObjectClouds = append(voxObjectClouds, seg.PointCloud)
	}
	voxColoredSegments, err := pc.MergePointCloudsWithColor(voxObjectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(voxColoredSegments, "gripper-segments-voxels")

	voxSegImage, err := segmentation.PointCloudSegmentsToMask(h.cameraParams.ColorCamera, voxObjectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(voxSegImage, "gripper-segmented-voxels-image-with-depth")

	return nil
}

func TestGripperVoxelObjectSegmentation(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper/color", "*.png", "segmentation/gripper/depth")
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(gripperComboParamsPath)
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperVoxelSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}
