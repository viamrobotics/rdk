package segmentation_test

import (
	"context"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/config"
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
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	// TODO(DATA-237): .both will be deprecated
	im := rimage.ConvertImage(img)
	dm, err := rimage.ConvertImageToDepthMap(img)
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
	voxObjConfig := config.AttributeMap{
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

	voxSegments, err := segmentation.RadiusClusteringFromVoxels(context.Background(), cam, voxObjConfig)
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
	objSegTest := os.Getenv(debugObjSeg)
	if objSegTest == "" {
		t.Skipf("set environmental variable %q to run this test", debugObjSeg)
	}
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper", "*.both.gz", true)
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/gripper_combo_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperVoxelSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}
