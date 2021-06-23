package pointcloud

import (
	"testing"

	"go.viam.com/test"

	"github.com/golang/geo/r3"
)

func TestCalculateMean(t *testing.T) {
	// create cloud
	cloud := New()
	p00 := NewBasicPoint(0, 0, 0)
	test.That(t, cloud.Set(p00), test.ShouldBeNil)
	p01 := NewBasicPoint(0, 0, 1)
	test.That(t, cloud.Set(p01), test.ShouldBeNil)
	p02 := NewBasicPoint(0, 1, 0)
	test.That(t, cloud.Set(p02), test.ShouldBeNil)
	p03 := NewBasicPoint(0, 1, 1)
	test.That(t, cloud.Set(p03), test.ShouldBeNil)

	mean0 := CalculateMeanOfPointCloud(cloud)
	test.That(t, mean0, test.ShouldResemble, r3.Vector{0, 0.5, 0.5})
}
