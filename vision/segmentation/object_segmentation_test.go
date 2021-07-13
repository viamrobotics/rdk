package segmentation

import (
	"bytes"
	"io/ioutil"
	"math"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/artifact"

	pc "go.viam.com/core/pointcloud"

	"github.com/golang/geo/r3"
)

func roundVector(v r3.Vector) r3.Vector {
	return r3.Vector{math.Round(v.X), math.Round(v.Y), math.Round(v.Z)}
}

// get a segmentation of a pointcloud and calculate each object's center
func TestCalculateSegmentMeans(t *testing.T) {
	// get file
	pcd, err := ioutil.ReadFile(artifact.MustPath("segmentation/aligned_intel/pointcloud-pieces.pcd"))
	test.That(t, err, test.ShouldBeNil)
	cloud, err := pc.ReadPCD(bytes.NewReader(pcd))
	test.That(t, err, test.ShouldBeNil)
	// do segmentation
	segments, err := CreateObjectSegmentation(cloud, 50000, 500, 10.0)
	test.That(t, err, test.ShouldBeNil)
	// get center points
	for i := 0; i < segments.N(); i++ {
		mean := pc.CalculateMeanOfPointCloud(segments.PointClouds[i])
		expMean := segments.Centers[i]
		test.That(t, mean, test.ShouldResemble, expMean)
	}
}
