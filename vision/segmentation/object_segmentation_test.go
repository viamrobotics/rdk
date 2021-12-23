package segmentation

import (
	"bytes"
	"context"
	"image"
	"io/ioutil"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/utils"

	"github.com/edaniels/golog"
)

// get a segmentation of a pointcloud and calculate each object's center
func TestCalculateSegmentMeans(t *testing.T) {
	// get file
	pcd, err := ioutil.ReadFile(artifact.MustPath("segmentation/aligned_intel/pointcloud-pieces.pcd"))
	test.That(t, err, test.ShouldBeNil)
	cloud, err := pc.ReadPCD(bytes.NewReader(pcd))
	test.That(t, err, test.ShouldBeNil)
	// do segmentation
	objConfig := ObjectConfig{
		MinPtsInPlane:    50000,
		MinPtsInSegment:  500,
		ClusteringRadius: 10.0,
	}
	segments, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)
	// get center points
	for i := 0; i < segments.N(); i++ {
		mean := pc.CalculateMeanOfPointCloud(segments.Objects[i].PointCloud)
		expMean := segments.Objects[i].Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}

// Test finding the objects in an aligned intel image
type segmentObjectTestHelper struct {
	cameraParams *transform.DepthColorIntrinsicsExtrinsics
}

// Process creates a segmentation using raw PointClouds and then VoxelGrids
func (h *segmentObjectTestHelper) Process(
	t *testing.T,
	pCtx *rimage.ProcessorContext,
	fn string,
	img image.Image,
	logger golog.Logger,
) error {
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	test.That(t, ii.IsAligned(), test.ShouldEqual, true)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)
	ii.SetProjector(h.cameraParams)

	pCtx.GotDebugImage(ii.Overlay(), "overlay")

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "depth-fixed")

	cloud, err := ii.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "intel-full-pointcloud")

	objConfig := ObjectConfig{
		MinPtsInPlane:    50000,
		MinPtsInSegment:  500,
		ClusteringRadius: 10.0,
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
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/aligned_intel", "*.both.gz", true)
	aligner, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/intel515_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &segmentObjectTestHelper{aligner})
	test.That(t, err, test.ShouldBeNil)
}

// Test finding objects in images from the gripper camera
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
	var err error
	ii := rimage.ConvertToImageWithDepth(img)
	test.That(t, h.cameraParams, test.ShouldNotBeNil)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth")

	// Pre-process the depth map to smooth the noise out and fill holes
	ii, err = rimage.PreprocessDepthMap(ii)
	test.That(t, err, test.ShouldBeNil)

	pCtx.GotDebugImage(ii.Depth.ToPrettyPicture(0, rimage.MaxDepth), "gripper-depth-filled")

	// Get the point cloud
	cloud, err := ii.ToPointCloud()
	test.That(t, err, test.ShouldBeNil)
	pCtx.GotDebugPointCloud(cloud, "gripper-pointcloud")

	// Do object segmentation with point clouds
	objConfig := ObjectConfig{
		MinPtsInPlane:    15000,
		MinPtsInSegment:  100,
		ClusteringRadius: 10.0,
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

	// turn pointclouds into voxel grid
	vg := pc.NewVoxelGridFromPointCloud(cloud, 5.0, 1.0)

	// Do voxel segmentation
	voxPlaneConfig := VoxelGridPlaneConfig{
		weightThresh:   0.9,
		angleThresh:    30,
		cosineThresh:   0.1,
		distanceThresh: 0.1,
	}
	voxObjConfig := ObjectConfig{
		MinPtsInPlane:    15000,
		MinPtsInSegment:  100,
		ClusteringRadius: 7.5,
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

func TestGripperObjectSegmentation(t *testing.T) {
	d := rimage.NewMultipleImageTestDebugger(t, "segmentation/gripper", "*.both.gz", true)
	camera, err := transform.NewDepthColorIntrinsicsExtrinsicsFromJSONFile(utils.ResolveFile("robots/configs/gripper_combo_parameters.json"))
	test.That(t, err, test.ShouldBeNil)

	err = d.Process(t, &gripperSegmentTestHelper{camera})
	test.That(t, err, test.ShouldBeNil)

}
