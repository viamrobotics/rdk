package slam

import (
	"context"
	"testing"

	"github.com/edaniels/test"
)

func TestAreaViewer(t *testing.T) {
	sa, err := NewSquareArea(100, 10)
	test.That(t, err, test.ShouldBeNil)
	viewer := AreaViewer{sa}

	sa.Mutate(func(area MutableArea) {
		area.Set(-250, 50, 1)
		area.Set(499, 90, 1)
		area.Set(222, 420, 1)
	})

	img, err := viewer.Next(context.Background())
	test.That(t, err, test.ShouldBeNil)
	bounds := img.Bounds()
	test.That(t, bounds.Dx(), test.ShouldEqual, 1000)
	test.That(t, bounds.Dy(), test.ShouldEqual, 1000)
	p1 := img.At(250, 550)
	p2 := img.At(999, 590)
	p3 := img.At(722, 920)
	r1, g1, b1, _ := p1.RGBA()
	r2, g2, b2, _ := p2.RGBA()
	r3, g3, b3, _ := p3.RGBA()

	test.That(t, r1>>8, test.ShouldEqual, 255)
	test.That(t, r2>>8, test.ShouldEqual, 255)
	test.That(t, r3>>8, test.ShouldEqual, 255)
	test.That(t, g1, test.ShouldEqual, 0)
	test.That(t, g2, test.ShouldEqual, 0)
	test.That(t, g3, test.ShouldEqual, 0)
	test.That(t, b1, test.ShouldEqual, 0)
	test.That(t, b2, test.ShouldEqual, 0)
	test.That(t, b3, test.ShouldEqual, 0)

	test.That(t, viewer.Close(), test.ShouldBeNil)
}
