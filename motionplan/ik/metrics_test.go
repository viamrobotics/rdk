package ik

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

func TestSqNormMetric(t *testing.T) {
	p1 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 0})
	p2 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})
	sqMet := NewSquaredNormMetric(p1)

	d1 := sqMet(&State{Position: p1})
	test.That(t, d1, test.ShouldAlmostEqual, 0)
	sqMet = NewSquaredNormMetric(p2)
	d2 := sqMet(&State{Position: p1})
	test.That(t, d2, test.ShouldAlmostEqual, 100)
}

func TestBasicMetric(t *testing.T) {
	sqMet := func(from, to spatial.Pose) float64 {
		return spatial.PoseDelta(from, to).Point().Norm2()
	}
	p1 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 0})
	p2 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})
	d1 := sqMet(p1, p1)
	test.That(t, d1, test.ShouldAlmostEqual, 0)
	d2 := sqMet(p1, p2)
	test.That(t, d2, test.ShouldAlmostEqual, 100)
}

var (
	ov     = &spatial.OrientationVector{math.Pi / 2, 0, 0, -1}
	p1b    = &State{Position: spatial.NewPose(r3.Vector{1, 2, 3}, ov)}
	p2b    = spatial.NewPose(r3.Vector{2, 3, 4}, ov)
	result float64
)

func BenchmarkDeltaPose1(b *testing.B) {
	var r float64
	weightedSqNormDist := NewSquaredNormMetric(p2b)
	for n := 0; n < b.N; n++ {
		r = weightedSqNormDist(p1b)
	}
	// Prevent compiler optimizations interfering with benchmark
	result = r
}
