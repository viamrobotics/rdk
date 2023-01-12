package transform

import (
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/spatialmath"
)

// Helper function that makes and returns a PointCloud from an artifact path.
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
	t.Run("Project a pointcloud with a null robot marker", func(t *testing.T) {
		_, err := NewParallelProjectionOntoXZWithRobotMarker(nil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "error null pointer given for robot position")
	})

	t.Run("Project empty an pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM, err := NewParallelProjectionOntoXZWithRobotMarker(&p)
		test.That(t, err, test.ShouldBeNil)

		pointcloud := pc.New()

		im, unusedImage, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err.Error(), test.ShouldContainSubstring, "calculation of the mean during pcd projection failed")
		test.That(t, im, test.ShouldBeNil)
		test.That(t, unusedImage, test.ShouldBeNil)
	})

	t.Run("Project a single point pointcloud with no data", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM, err := NewParallelProjectionOntoXZWithRobotMarker(&p)
		test.That(t, err, test.ShouldBeNil)

		pointcloud := pc.New()
		pointcloud.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewBasicData())

		im, unusedImage, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedImage, test.ShouldBeNil)
	})

	t.Run("Project a single point pointcloud with data", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM, err := NewParallelProjectionOntoXZWithRobotMarker(&p)
		test.That(t, err, test.ShouldBeNil)

		pointcloud := pc.New()
		pointcloud.Set(r3.Vector{X: 0, Y: 0, Z: 0}, pc.NewValueData(1))

		im, unusedImage, err := ppRM.PointCloudToRGBD(pointcloud)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedImage, test.ShouldBeNil)
	})

	t.Run("Project an imported pointcloud", func(t *testing.T) {
		p := spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, spatialmath.NewOrientationVector())
		ppRM, err := NewParallelProjectionOntoXZWithRobotMarker(&p)
		test.That(t, err, test.ShouldBeNil)

		startPC, err := makePointCloudFromArtifact(t, "pointcloud/test.pcd", 100)
		test.That(t, err, test.ShouldBeNil)

		im, unusedImage, err := ppRM.PointCloudToRGBD(startPC)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, im.Width(), test.ShouldEqual, imageWidth)
		test.That(t, im.Height(), test.ShouldEqual, imageHeight)
		test.That(t, unusedImage, test.ShouldBeNil)
	})
}
