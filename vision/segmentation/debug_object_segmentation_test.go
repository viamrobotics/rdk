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

const debugObjSeg = "VIAM_DEBUG"

// Test finding the objects in an aligned intel image.
type segmentObjectTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

// Process creates a segmentation using raw PointClouds and then VoxelGrids.
func (h *segmentObjectTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	t.Helper()
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	test.That(t, ii.IsAligned(), test.ShouldEqual, true)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	pCtx.GotDebugImage(ii.Overlay(), "overlay")

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	cloud, err := h.cameraParams.ImageWithDepthToPointCloud(ii)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "intel-full-pointcloud")

	objConfig := &vision.Parameters3D{
		MinPtsInPlane:      50000,
		MinPtsInSegment:    500,
		ClusteringRadiusMm: 10.0,
	}

	// Do object segmentation with point clouds
	segments, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)

	objectClouds := segments.PointClouds()
	coloredSegments, err := pc.MergePointCloudsWithColor(objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(coloredSegments, "intel-segments-pointcloud")

	segImage, err := PointCloudSegmentsToMask(h.cameraParams.ColorCamera, objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(segImage, "segmented-pointcloud-image-with-depth")

	return nil
}

func TestObjectSegmentationAlignedIntel(t *testing.T) {
	objSegTest := os.Getenv(debugObjSeg)
	if objSegTest == "" {
		t.Skip(fmt.Sprintf("set environmental variable %q to run this test", debugObjSeg))
	}
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel", "*.both.gz", true)
	aligner, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
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

	// Do object segmentation with point clouds
	objConfig := &vision.Parameters3D{
		MinPtsInPlane:      15000,
		MinPtsInSegment:    100,
		ClusteringRadiusMm: 10.0,
	}

	segments, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)

	objectClouds := segments.PointClouds()
	coloredSegments, err := pc.MergePointCloudsWithColor(objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(coloredSegments, "gripper-segments-pointcloud")

	segImage, err := PointCloudSegmentsToMask(h.cameraParams.ColorCamera, objectClouds)
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugImage(segImage, "gripper-segmented-pointcloud-image-with-depth")

	return nil
}

func TestGripperObjectSegmentation(t *testing.T) {
	objSegTest := os.Getenv(debugObjSeg)
	if objSegTest == "" {
		t.Skip(fmt.Sprintf("set environmental variable %q to run this test", debugObjSeg))
	}
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper", "*.both.gz", true)
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/gripper_combo_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)
}
