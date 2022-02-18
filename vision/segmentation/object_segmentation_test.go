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
func TestCalculateSegmentMeans(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cloud, err := pc.NewFromLASFile(artifact.MustPath("pointcloud/test.las"), logger)
	test.That(t, err, test.ShouldBeNil)
	// do segmentation
	objConfig := &vision.Parameters3D{
		MinPtsInPlane:      50000,
		MinPtsInSegment:    500,
		ClusteringRadiusMm: 10.0,
	}
	segments, err := NewObjectSegmentation(context.Background(), cloud, objConfig)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(segments.Objects()), test.ShouldBeGreaterThan, 0)
	// get center points
	for i := 0; i < segments.N(); i++ {
		mean := pc.CalculateMeanOfPointCloud(segments.Segments.Objects[i].PointCloud)
		expMean := segments.Segments.Objects[i].Center
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}
