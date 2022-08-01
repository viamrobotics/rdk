package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestMatrixStorage(t *testing.T) {
	ms := matrixStorage{points: make([]PointAndData, 0), indexMap: make(map[r3.Vector]uint)}
	test.That(t, ms.IsOrdered(), test.ShouldEqual, true)
	testPointCloudStorage(t, &ms)
}

func BenchmarkMatrixStorage(b *testing.B) {
	ms := matrixStorage{points: make([]PointAndData, 0), indexMap: make(map[r3.Vector]uint)}
	benchPointCloudStorage(b, &ms)
}
