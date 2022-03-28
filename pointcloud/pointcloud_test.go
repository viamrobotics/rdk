package pointcloud

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
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
