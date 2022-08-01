package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestMapStorage(t *testing.T) {
	ms := mapStorage{points: make(map[r3.Vector]Data)}
	test.That(t, ms.IsOrdered(), test.ShouldEqual, false)
	testPointCloudStorage(t, &ms)
}
