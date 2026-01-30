package motionplan

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestSqNormMetric(t *testing.T) {
	p1 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 0})
	p2 := spatial.NewPoseFromPoint(r3.Vector{0, 0, 10})

	// Test using WeightedSquaredNormDistance function
	d1 := WeightedSquaredNormDistance(p1, p1)
	test.That(t, d1, test.ShouldAlmostEqual, 0)
	d2 := WeightedSquaredNormDistance(p1, p2)
	test.That(t, d2, test.ShouldAlmostEqual, 1.0) // 10^2 * 0.01 = 1.0
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
	p1b    = spatial.NewPose(r3.Vector{1, 2, 3}, ov)
	p2b    = spatial.NewPose(r3.Vector{2, 3, 4}, ov)
	result float64
)

func BenchmarkDeltaPose1(b *testing.B) {
	var r float64
	for n := 0; n < b.N; n++ {
		r = WeightedSquaredNormDistance(p1b, p2b)
	}
	// Prevent compiler optimizations interfering with benchmark
	result = r
}

func TestFSDisplacementDistance(t *testing.T) {
	armModel, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("test")
	test.That(t, fs.AddFrame(armModel, fs.World()), test.ShouldBeNil)

	makeSegment := func(start, end []referenceframe.Input) *SegmentFS {
		return &SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{"xarm6": start}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{"xarm6": end}.ToLinearInputs(),
			FS:                 fs,
		}
	}

	home := []referenceframe.Input{0, 0, 0, 0, 0, 0}

	t.Run("zero movement", func(t *testing.T) {
		test.That(t, FSDisplacementDistance(makeSegment(home, home)), test.ShouldAlmostEqual, 0)
	})

	t.Run("larger movement produces more displacement", func(t *testing.T) {
		small := FSDisplacementDistance(makeSegment(home, []referenceframe.Input{0.1, 0, 0, 0, 0, 0}))
		large := FSDisplacementDistance(makeSegment(home, []referenceframe.Input{0.5, 0, 0, 0, 0, 0}))
		test.That(t, small, test.ShouldBeGreaterThan, 0)
		test.That(t, large, test.ShouldBeGreaterThan, small)
	})

	t.Run("shoulder vs wrist movement", func(t *testing.T) {
		wrist := FSDisplacementDistance(makeSegment(home, []referenceframe.Input{0, 0, 0, 0, 0.6, 0.6}))
		shoulder := FSDisplacementDistance(makeSegment(home, []referenceframe.Input{0, 0.5, 0, 0, 0, 0}))
		test.That(t, shoulder, test.ShouldBeGreaterThan, wrist)
	})
}
