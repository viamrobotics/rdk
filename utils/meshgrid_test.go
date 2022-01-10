package utils

import (
	"testing"

	"go.viam.com/test"
)

func TestSingle(t *testing.T) {
	x := make([]float64, 3)
	x[0] = 0
	x[1] = 1
	x[2] = 2
	mesh := Single(2, x)
	test.That(t, len(mesh), test.ShouldEqual, 9)
	test.That(t, len(mesh[0]), test.ShouldEqual, 2)
	test.That(t, mesh[0][0], test.ShouldEqual, 0)
	test.That(t, mesh[0][1], test.ShouldEqual, 0)
	test.That(t, mesh[8][0], test.ShouldEqual, 2)
	test.That(t, mesh[8][1], test.ShouldEqual, 2)
}
