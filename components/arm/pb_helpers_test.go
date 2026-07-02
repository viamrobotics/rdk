package arm

import (
	"math"
	"testing"
	"time"

	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

// TestTrajectoryPointProtoRoundTrip exercises the converters against a real all-revolute arm model
// and pins the unit-consistency fix: positions, velocities, and accelerations all cross the wire in
// degrees, and round-trip back to radians.
func TestTrajectoryPointProtoRoundTrip(t *testing.T) {
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("referenceframe/testfiles/ur5e.json"), "foo")
	test.That(t, err, test.ShouldBeNil)

	dof := len(model.DoF())
	positions := make([]referenceframe.Input, dof)
	velocities := make([]float64, dof)
	accelerations := make([]float64, dof)
	for i := range positions {
		positions[i] = math.Pi / 4  // -> 45 deg
		velocities[i] = math.Pi / 2 // -> 90 deg/s
		accelerations[i] = math.Pi  // -> 180 deg/s^2
	}
	point := TrajectoryPoint{
		Time:      250 * time.Millisecond,
		Positions: positions,
		Constraints: &KinematicConstraints{
			Velocities:    velocities,
			Accelerations: accelerations,
		},
	}

	pbPoint, err := trajectoryPointToProto(model, point)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pbPoint.GetTime().AsDuration(), test.ShouldEqual, 250*time.Millisecond)
	test.That(t, pbPoint.GetPositions().GetValues()[0], test.ShouldAlmostEqual, 45.0)
	test.That(t, pbPoint.GetConstraints().GetVelocities().GetValues()[0], test.ShouldAlmostEqual, 90.0)
	test.That(t, pbPoint.GetConstraints().GetAccelerations().GetValues()[0], test.ShouldAlmostEqual, 180.0)

	back, err := trajectoryPointFromProto(model, pbPoint)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back.Time, test.ShouldEqual, point.Time)
	for i := range positions {
		test.That(t, back.Positions[i], test.ShouldAlmostEqual, positions[i])
		test.That(t, back.Constraints.Velocities[i], test.ShouldAlmostEqual, velocities[i])
		test.That(t, back.Constraints.Accelerations[i], test.ShouldAlmostEqual, accelerations[i])
	}
}

// TestTrajectoryPointProtoOptionalFields covers the nil-model fallback and the optional Constraints
// and Accelerations fields.
func TestTrajectoryPointProtoOptionalFields(t *testing.T) {
	// No constraints at all.
	point := TrajectoryPoint{
		Time:      time.Second,
		Positions: []referenceframe.Input{0, math.Pi},
	}
	pbPoint, err := trajectoryPointToProto(nil, point)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pbPoint.GetConstraints(), test.ShouldBeNil)

	back, err := trajectoryPointFromProto(nil, pbPoint)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back.Constraints, test.ShouldBeNil)
	test.That(t, back.Positions, test.ShouldResemble, point.Positions)

	// Constraints present, accelerations absent.
	point.Constraints = &KinematicConstraints{Velocities: []float64{0, math.Pi}}
	pbPoint, err = trajectoryPointToProto(nil, point)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pbPoint.GetConstraints().GetVelocities().GetValues()[1], test.ShouldAlmostEqual, 180.0)
	test.That(t, pbPoint.GetConstraints().GetAccelerations(), test.ShouldBeNil)

	back, err = trajectoryPointFromProto(nil, pbPoint)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, back.Constraints.Accelerations, test.ShouldBeNil)
	test.That(t, back.Constraints.Velocities, test.ShouldResemble, []float64{0, math.Pi})
}
