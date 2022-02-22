package segmentation

import (
	"context"
	"fmt"
	"image"
	"os"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
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
	ii := rimage.ConvertToImageWithDepth(img)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth")

	// Pre-process the depth map to smooth the noise out and fill holes
	ii, err = rimage.PreprocessDepthMap(ii)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth-filled")

	// Get the point cloud
	cloud, err := h.cameraParams.ImageWithDepthToPointCloud(ii)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "gripper-pointcloud")

	// turn pointclouds into voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, 5.0, 1.0)

	// Do voxel segmentation
	voxPlaneConfig := VoxelGridPlaneConfig{
		weightThresh:   0.9,
		angleThresh:    30,
		cosineThresh:   0.1,
		distanceThresh: 0.1,
	}
	voxObjConfig := &vision.Parameters3D{
		MinPtsInPlane:      15000,
		MinPtsInSegment:    100,
		ClusteringRadiusMm: 7.5,
	}

	voxSegments, err := NewObjectSegmentationFromVoxelGrid(context.Background(), vg, voxObjConfig, voxPlaneConfig)
	test.That(t, err, test.ShouldBeNil)

	voxObjectClouds := voxSegments.PointClouds()
	voxColoredSegments, err := pc.MergePointCloudsWithColor(voxObjectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(voxColoredSegments, "gripper-segments-voxels")

	voxSegImage, err := PointCloudSegmentsToMask(h.cameraParams.ColorCamera, voxObjectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(voxSegImage, "gripper-segmented-voxels-image-with-depth")

	return nil
}

func TestGripperVoxelObjectSegmentation(t *testing.T) {
	objSegTest := os.Getenv(debugObjSeg)
	if objSegTest == "" {
		t.Skip(fmt.Sprintf("set environmental variable %q to run this test", debugObjSeg))
	}
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper", "*.both.gz", true)
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/gripper_combo_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperVoxelSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}
