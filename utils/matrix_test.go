package utils

import (
	"math/rand"
	"testing"

	"github.com/edaniels/test"
	"gonum.org/v1/gonum/mat"
)

func TestDistanceMSETo(t *testing.T) {
	cols := 360
	left := mat.NewDense(3, cols, nil)
	for i := 0; i < cols; i++ {
		left.Set(0, i, rand.Float64())
		left.Set(1, i, rand.Float64())
		left.Set(2, i, 1)
	}
	leftVec2 := (*Vec2Matrix)(left)
	dist := leftVec2.DistanceMSETo(leftVec2)
	test.That(t, dist, test.ShouldEqual, 0)

	rot := leftVec2.RotateMatrixAbout(0, 0, 0)
	dist = leftVec2.DistanceMSETo(rot)
	test.That(t, dist, test.ShouldEqual, 0)

	rot = leftVec2.RotateMatrixAbout(0, 0, 45)
	dist = leftVec2.DistanceMSETo(rot)
	test.That(t, dist, test.ShouldNotEqual, 0)
}
