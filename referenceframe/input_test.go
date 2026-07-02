package referenceframe

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
)

func TestJointPositions(t *testing.T) {
	in := []float64{0, math.Pi}
	j := JointPositionsFromRadians(in)
	test.That(t, j.Values[0], test.ShouldEqual, 0.0)
	test.That(t, j.Values[1], test.ShouldEqual, 180.0)
	test.That(t, JointPositionsToRadians(j), test.ShouldResemble, in)
}

// TestJointVelocities and TestJointAccelerations pin the invariant that velocities and
// accelerations get the same revolute radians<->degrees scaling as positions. A nil frame assumes
// all-revolute, so radians/second must become degrees/second (and second^2 likewise) on the wire.
func TestJointVelocities(t *testing.T) {
	in := []Input{0, math.Pi}
	jv, err := JointVelocitiesFromInputs(nil, in)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jv.Values[0], test.ShouldEqual, 0.0)
	test.That(t, jv.Values[1], test.ShouldEqual, 180.0)

	back, err := InputsFromJointVelocities(nil, jv)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back, test.ShouldResemble, in)
}

func TestJointAccelerations(t *testing.T) {
	in := []Input{0, math.Pi}
	ja, err := JointAccelerationsFromInputs(nil, in)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ja.Values[0], test.ShouldEqual, 0.0)
	test.That(t, ja.Values[1], test.ShouldEqual, 180.0)

	back, err := InputsFromJointAccelerations(nil, ja)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back, test.ShouldResemble, in)
}

// TestJointDerivativesFrameAware confirms the per-joint dispatch: a revolute joint scales
// radians<->degrees while a prismatic joint passes millimeters through unchanged.
func TestJointDerivativesFrameAware(t *testing.T) {
	rev, err := NewRotationalFrame("rev", spatial.R4AA{RZ: 1}, Limit{Min: -2 * math.Pi, Max: 2 * math.Pi})
	test.That(t, err, test.ShouldBeNil)
	jv, err := JointVelocitiesFromInputs(rev, []Input{math.Pi})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jv.Values[0], test.ShouldAlmostEqual, 180.0)

	prism, err := NewTranslationalFrame("prism", r3.Vector{X: 1}, Limit{Min: -100, Max: 100})
	test.That(t, err, test.ShouldBeNil)
	jv, err = JointVelocitiesFromInputs(prism, []Input{50})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, jv.Values[0], test.ShouldEqual, 50.0)
}

func TestInterpolateValues(t *testing.T) {
	jp1 := []Input{0, 4}
	jp2 := []Input{8, -8}
	jpHalf := []Input{4, -2}
	jpQuarter := []Input{2, 1}

	interp1 := interpolateInputs(jp1, jp2, 0.5)
	interp2 := interpolateInputs(jp1, jp2, 0.25)
	test.That(t, interp1, test.ShouldResemble, jpHalf)
	test.That(t, interp2, test.ShouldResemble, jpQuarter)
}
