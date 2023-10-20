package motionplan

import (
	"math"
	"testing"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

func TestFixOvIncrement(t *testing.T) {
	pos1 := commonpb.Pose{
		X:     -66,
		Y:     -133,
		Z:     372,
		Theta: 15,
		OX:    0,
		OY:    1,
		OZ:    0,
	}
	pos2 := commonpb.Pose{
		X:     pos1.X,
		Y:     pos1.Y,
		Z:     pos1.Z,
		Theta: pos1.Theta,
		OX:    pos1.OX,
		OY:    pos1.OY,
		OZ:    pos1.OZ,
	}

	// Increment, but we're not pointing at Z axis, so should do nothing
	pos2.OX = -0.1
	outpos := fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// point at positive Z axis, decrement OX, should subtract 180
	pos1.OZ = 1
	pos2.OZ = 1
	pos1.OY = 0
	pos2.OY = 0
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -165)

	// Spatial translation is incremented, should do nothing
	pos2.X -= 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))

	// Point at -Z, increment OY
	pos2.X += 0.1
	pos2.OX += 0.1
	pos1.OZ = -1
	pos2.OZ = -1
	pos2.OY = 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos.Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 105)

	// OX and OY are both incremented, should do nothing
	pos2.OX += 0.1
	outpos = fixOvIncrement(spatialmath.NewPoseFromProtobuf(&pos2), spatialmath.NewPoseFromProtobuf(&pos1))
	test.That(t, outpos, test.ShouldResemble, spatialmath.NewPoseFromProtobuf(&pos2))
}

func TestEvaluate(t *testing.T) {
	plan := Plan{
		map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}},
	}
	score := plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Inf(1))

	// Test no change
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, 0)

	// Test L2 for "", and nothing for plan with only one entry
	plan = append(plan, map[string][]referenceframe.Input{"": {{4.}, {5.}, {6.}}, "test": {{2.}, {3.}, {4.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27))

	// Test cumulative L2 after returning to original inputs
	plan = append(plan, map[string][]referenceframe.Input{"": {{1.}, {2.}, {3.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2)

	// Test that the "test" inputs are properly evaluated after skipping a step
	plan = append(plan, map[string][]referenceframe.Input{"test": {{3.}, {5.}, {6.}}})
	score = plan.Evaluate(ik.L2InputMetric)
	test.That(t, score, test.ShouldEqual, math.Sqrt(27)*2+3)

	// Evaluated with the tp-space metric, should be the sum of the distance values (third input) ignoring the first input set for each
	// named input set
	score = plan.Evaluate(tpspace.PTGSegmentMetric)
	test.That(t, score, test.ShouldEqual, 18)
}
