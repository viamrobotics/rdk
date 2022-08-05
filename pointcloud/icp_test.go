package pointcloud

import (
	"os"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/spatialmath"
)

func pointCloudFromArtifact(t *testing.T, artifactPath string) (PointCloud, error) {
	t.Helper()
	pcdFile, err := os.Open(artifact.MustPath(artifactPath))
	if err != nil {
		return nil, err
	}
	pc, err := ReadPCD(pcdFile)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

func TestICPRegistration(t *testing.T) {
	if os.Getenv("VIAM_DEBUG") == "" {
		t.Skip("Test is too large for now.")
	}

	targetCloud, err := pointCloudFromArtifact(t, "pointcloud/bun000.pcd")
	targetKD := ToKDTree(targetCloud)
	test.That(t, err, test.ShouldBeNil)

	sourceCloud, err := pointCloudFromArtifact(t, "pointcloud/bun045.pcd")
	test.That(t, err, test.ShouldBeNil)

	guess := spatialmath.NewPoseFromOrientation(
		r3.Vector{-60, 0, -10},
		&spatialmath.EulerAngles{
			Roll:  0,
			Pitch: 0.6,
			Yaw:   0,
		})
	registered, info, err := RegisterPointCloudICP(sourceCloud, targetKD, guess, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, registered, test.ShouldNotBeNil)
	test.That(t, info, test.ShouldNotBeNil)

	test.That(t, info.OptResult.F, test.ShouldBeLessThan, 20.)
}
