package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

func TestPointCloudBasic(t *testing.T) {
	pc := New()

	p0 := NewVector(0, 0, 0)
	d0 := NewValueData(5)

	test.That(t, pc.Set(p0, d0), test.ShouldBeNil)
	d, got := pc.At(0, 0, 0)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, d, test.ShouldResemble, d0)

	_, got = pc.At(1, 0, 1)
	test.That(t, got, test.ShouldBeFalse)

	p1 := NewVector(1, 0, 1)
	d1 := NewValueData(17)
	test.That(t, pc.Set(p1, d1), test.ShouldBeNil)

	d, got = pc.At(1, 0, 1)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, d, test.ShouldResemble, d1)
	test.That(t, d, test.ShouldNotResemble, d0)

	p2 := NewVector(-1, -2, 1)
	d2 := NewValueData(81)
	test.That(t, pc.Set(p2, d2), test.ShouldBeNil)
	d, got = pc.At(-1, -2, 1)
	test.That(t, got, test.ShouldBeTrue)
	test.That(t, d, test.ShouldResemble, d2)

	count := 0
	pc.Iterate(0, 0, func(p r3.Vector, d Data) bool {
		switch p.X {
		case 0:
			test.That(t, p, test.ShouldResemble, p0)
		case 1:
			test.That(t, p, test.ShouldResemble, p1)
		case -1:
			test.That(t, p, test.ShouldResemble, p2)
		}
		count++
		return true
	})
	test.That(t, count, test.ShouldEqual, 3)

	test.That(t, CloudContains(pc, 1, 1, 1), test.ShouldBeFalse)

	pMax := NewVector(minPreciseFloat64, maxPreciseFloat64, minPreciseFloat64)
	test.That(t, pc.Set(pMax, nil), test.ShouldBeNil)

	pBad := NewVector(minPreciseFloat64-1, maxPreciseFloat64, minPreciseFloat64)
	err := pc.Set(pBad, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "x component")

	pBad = NewVector(minPreciseFloat64, maxPreciseFloat64+1, minPreciseFloat64)
	err = pc.Set(pBad, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "y component")

	pBad = NewVector(minPreciseFloat64, maxPreciseFloat64, minPreciseFloat64-1)
	err = pc.Set(pBad, nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "z component")
}

func TestPointCloudCentroid(t *testing.T) {
	var point r3.Vector
	var data Data
	pc := New()

	test.That(t, pc.Size(), test.ShouldResemble, 0)
	test.That(t, CloudCentroid(pc), test.ShouldResemble, r3.Vector{0, 0, 0})

	point = NewVector(10, 100, 1000)
	data = NewValueData(1)
	test.That(t, pc.Set(point, data), test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldResemble, 1)
	test.That(t, CloudCentroid(pc), test.ShouldResemble, point)

	point = NewVector(20, 200, 2000)
	data = NewValueData(2)
	test.That(t, pc.Set(point, data), test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldResemble, 2)
	test.That(t, CloudCentroid(pc), test.ShouldResemble, r3.Vector{15, 150, 1500})

	point = NewVector(30, 300, 3000)
	data = NewValueData(3)
	test.That(t, pc.Set(point, data), test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldResemble, 3)
	test.That(t, CloudCentroid(pc), test.ShouldResemble, r3.Vector{20, 200, 2000})

	point = NewVector(30, 300, 3000)
	data = NewValueData(3)
	test.That(t, pc.Set(point, data), test.ShouldBeNil)
	test.That(t, pc.Size(), test.ShouldResemble, 3)
	test.That(t, CloudCentroid(pc), test.ShouldResemble, r3.Vector{20, 200, 2000})
}

func TestPointCloudMatrix(t *testing.T) {
	pc := New()

	p := NewVector(0, 0, 0)
	d := NewValueData(5)

	test.That(t, pc.Set(p, d), test.ShouldBeNil)
	p = NewVector(1, 2, 3)
	d = NewValueData(4)
	test.That(t, pc.Set(p, d), test.ShouldBeNil)

	refMatrix := mat.NewDense(2, 4, []float64{0, 0, 0, 5, 1, 2, 3, 4})
	m, h := CloudMatrix(pc)
	test.That(t, h, test.ShouldResemble, []CloudMatrixCols{CloudMatrixColX, CloudMatrixColY, CloudMatrixColZ, CloudMatrixColV})
	test.That(t, m, test.ShouldResemble, refMatrix)
}
