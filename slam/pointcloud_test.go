package slam

import (
	"testing"

	"github.com/edaniels/test"
)

func TestPointCloudBasic(t *testing.T) {
	pc := NewPointCloud()
	p0 := NewPoint(0, 0, 0)
	pc.Set(p0)
	pAt := pc.At(0, 0, 0)
	test.That(t, pAt, test.ShouldResemble, p0)
	p1 := NewPoint(1, 0, 1)
	pc.Set(p1)
	pAt = pc.At(1, 0, 1)
	test.That(t, pAt, test.ShouldResemble, p1)
	test.That(t, pAt, test.ShouldNotResemble, p0)

	count := 0
	pc.Iterate(func(p Point) bool {
		switch p.Position().X {
		case 0:
			test.That(t, p, test.ShouldResemble, p0)
		case 1:
			test.That(t, p, test.ShouldResemble, p1)
		}
		count++
		return true
	})
	test.That(t, count, test.ShouldEqual, 2)

	test.That(t, pc.At(1, 1, 1), test.ShouldBeNil)
}
