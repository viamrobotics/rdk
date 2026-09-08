package spatialmath

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestR3VectorAlmostEqual(t *testing.T) {
	test.That(t, R3VectorAlmostEqual(r3.Vector{1, 2, 3}, r3.Vector{1.001, 2, 3}, 1e-4), test.ShouldBeFalse)
	test.That(t, R3VectorAlmostEqual(r3.Vector{1, 2, 3}, r3.Vector{1.001, 2.001, 3.001}, 1e-2), test.ShouldBeTrue)
}

func TestCentroidOfPoints(t *testing.T) {
	test.That(t, CentroidOfPoints(nil), test.ShouldResemble, r3.Vector{})
	test.That(t, CentroidOfPoints([]r3.Vector{}), test.ShouldResemble, r3.Vector{})
	test.That(t, CentroidOfPoints([]r3.Vector{{1, 2, 3}}), test.ShouldResemble, r3.Vector{1, 2, 3})
	test.That(t,
		CentroidOfPoints([]r3.Vector{{0, 0, 0}, {2, 4, 6}, {4, 8, 12}}),
		test.ShouldResemble,
		r3.Vector{2, 4, 6},
	)
}

func TestAxisSerialization(t *testing.T) {
	tc := NewAxisConfig(R4AA{Theta: 1, RX: 1})
	newTc := NewAxisConfig(tc.ParseConfig())
	test.That(t, tc, test.ShouldResemble, newTc)
}
