package transform

import (
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/rimage"
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
	p := spatialmath.NewPose(r3.Vector{0, 0, 0}, spatialmath.NewOrientationVector())
	ppRM := NewParallelProjectionOntoXZWithRobotMarker(&p)

	startPC, err := makePointCloudFromArtifact(t, "pointcloud/test.pcd", 100)
	test.That(t, err, test.ShouldBeNil)

	im, _, err := ppRM.PointCloudToRGBD(startPC)
	test.That(t, err, test.ShouldBeNil)

	err = rimage.WriteImageToFile("test_image_ppRM.png", im)
	test.That(t, err, test.ShouldBeNil)
}
