package pointcloud

import (
	"testing"

	"github.com/edaniels/test"
)

func TestPointCloudBasic(t *testing.T) {
	pc := New()
	p0 := NewPoint(0, 0, 0)
	test.That(t, pc.Set(p0), test.ShouldBeNil)
	pAt := pc.At(0, 0, 0)
	test.That(t, pAt, test.ShouldResemble, p0)
	p1 := NewPoint(1, 0, 1)
	test.That(t, pc.Set(p1), test.ShouldBeNil)
	pAt = pc.At(1, 0, 1)
	test.That(t, pAt, test.ShouldResemble, p1)
	test.That(t, pAt, test.ShouldNotResemble, p0)
	p2 := NewPoint(-1, -2, 1)
	test.That(t, pc.Set(p2), test.ShouldBeNil)
	pAt = pc.At(-1, -2, 1)
	test.That(t, pAt, test.ShouldResemble, p2)

	count := 0
	pc.Iterate(func(p Point) bool {
		switch p.Position().X {
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

	test.That(t, pc.At(1, 1, 1), test.ShouldBeNil)

	pMax := NewPoint(minExactFloat64Integer, maxExactFloat64Integer, minExactFloat64Integer)
	test.That(t, pc.Set(pMax), test.ShouldBeNil)

	pBad := NewPoint(minExactFloat64Integer-1, maxExactFloat64Integer, minExactFloat64Integer)
	err := pc.Set(pBad)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "x component")

	pBad = NewPoint(minExactFloat64Integer, maxExactFloat64Integer+1, minExactFloat64Integer)
	err = pc.Set(pBad)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "y component")

	pBad = NewPoint(minExactFloat64Integer, maxExactFloat64Integer, minExactFloat64Integer-1)
	err = pc.Set(pBad)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "z component")
}
