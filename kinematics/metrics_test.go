package kinematics

import (
	"testing"

	spatial "go.viam.com/core/spatialmath"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
)

func TestDefaultMetric(t *testing.T) {
	sqMet := NewSquaredNormMetric()

	p1 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 0})
	p2 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})

	d1 := sqMet(p1, p1)
	test.That(t, d1, test.ShouldAlmostEqual, 0)
	d2 := sqMet(p1, p2)
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
