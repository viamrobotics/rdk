package pointcloud

import (
	"image/color"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func testPointCloudStorage(t *testing.T, ms storage) {
	t.Helper()
	var point r3.Vector
	var data, gotData Data
	var found bool
	// Empty
	test.That(t, ms.Size(), test.ShouldEqual, 0)

	// Insertion
	point = r3.Vector{1, 2, 3}
	data = NewColoredData(color.NRGBA{255, 124, 43, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 1)
	gotData, found = ms.At(1, 2, 3)
	test.That(t, found, test.ShouldEqual, true)
	test.That(t, gotData, test.ShouldEqual, data)

	// Second Insertion
	point = r3.Vector{4, 2, 3}
	data = NewColoredData(color.NRGBA{232, 111, 75, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 2)

	// Insertion of duplicate point
	data = NewColoredData(color.NRGBA{22, 1, 78, 255})
	test.That(t, ms.Set(point, data), test.ShouldEqual, nil)
	test.That(t, ms.Size(), test.ShouldEqual, 2)
	gotData, found = ms.At(4, 2, 3)
	test.That(t, found, test.ShouldEqual, true)
	test.That(t, gotData, test.ShouldEqual, data)

	// Retrieval of non-existent point
	gotData, found = ms.At(3, 1, 7)
	test.That(t, found, test.ShouldEqual, false)
	test.That(t, gotData, test.ShouldBeNil)
}
