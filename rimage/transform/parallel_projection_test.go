package transform

import (
	"image"
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/spatialmath"
)

// Helper function that makes and returns a PointCloud from an artifact path consisting of the first numPoints of points.
func makePointCloudFromArtifact(t *testing.T, artifactPath string, numPoints int) (pc.PointCloud, error) {
	t.Helper()
	pcdFile, err := os.Open(artifact.MustPath(artifactPath))
	if err != nil {
		return nil, err
	}
	pcd, err := pc.ReadPCD(pcdFile)
	if err != nil {
		return nil, err
	}

	if numPoints == 0 {
		return pcd, nil
	}

	shortenedPC := pc.NewWithPrealloc(numPoints)

	counter := numPoints
	pcd.Iterate(0, 0, func(p r3.Vector, d pc.Data) bool {
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

func TestParallelProjectionOntoXZWithRobotMarker(t *testing.T) {
	t.Run("Project an empty pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err.Error(), test.ShouldContainSubstring, "calculation of the X-coord's mean during pcd projection failed")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a single point pointcloud with no data", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()
		pointcloud.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewBasicData())

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a point with out of range data", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()
		pointcloud.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(200))

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error parallel projection with robot marker received a value of")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})

	t.Run("Project a single point pointcloud with data with image pixel checks", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		pointcloud := pc.New()
		d := pc.NewBasicData()
		pointcloud.Set(r3.Vector{X: 10, Y: 10, Z: 10}, d)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)

		robotMarkerExpectedPos := image.Point{X: 0, Y: 0}
		colorAtPos := im.GetXY(robotMarkerExpectedPos.X, robotMarkerExpectedPos.Y)
		expectedRobotMarkerColor := rimage.NewColor(255, 0, 0)
		test.That(t, colorAtPos, test.ShouldResemble, expectedRobotMarkerColor)

		pointExpectedPos := image.Point{X: imageHeight - 1, Y: imageWidth - 1}
		colorAtPoint := im.GetXY(pointExpectedPos.X, pointExpectedPos.Y)
		expectedPointColor, err := getProbabilityColorFromValue(d)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, colorAtPoint, test.ShouldResemble, expectedPointColor)
	})

	t.Run("Project an imported pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

		startPC, err := makePointCloudFromArtifact(t, "pointcloud/test.pcd", 100)
		test.That(t, err, test.ShouldBeNil)

		im, unusedDepthMap, err := ppRM.PointCloudToRGBD(startPC)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedDepthMap, test.ShouldBeNil)
	})
}
