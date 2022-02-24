package segmentation

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/vision"
)

// get a segmentation of a pointcloud and calculate each object's center.
func TestPixelSegmentation(t *testing.T) {
	cloud := loadPointCloud(t)

	// do segmentation
	objConfig := &vision.Parameters3D{
		MinPtsInPlane:      50000,
		MinPtsInSegment:    500,
		ClusteringRadiusMm: 10.0,
	}
	segmentation, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)
	testSegmentation(t, segmentation)
}

func loadPointCloud(t *testing.T) pc.PointCloud {
	t.Helper()
	logger := golog.NewTestLogger(t)
	cloud, err := pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	return cloud
}

func testSegmentation(t *testing.T, segmentation *ObjectSegmentation) {
	t.Helper()
	test.That(t, segmentation.N(), test.ShouldBeGreaterThan, 0)
	for i := 0; i < segmentation.N(); i++ {
		box, err := pc.BoundingBoxFromPointCloud(segmentation.Segments.Objects[i])
		if segmentation.Segments.Objects[i].Size() == 0 {
			test.That(t, box, test.ShouldBeNil)
			test.That(t, err, test.ShouldNotBeNil)
			continue
		}
		test.That(t, box, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, box.AlmostEqual(segmentation.Segments.Objects[i].BoundingBox), test.ShouldBeTrue)
	}
}
