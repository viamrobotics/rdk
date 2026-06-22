package referenceframe

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
)

func TestPoseInFrame(t *testing.T) {
	pose := spatialmath.NewPose(r3.Vector{1, 2, 3}, &spatialmath.OrientationVector{math.Pi / 2, 0, 0, -1})
	pF := NewPoseInFrame("frame", pose)
	test.That(t, pF.Parent(), test.ShouldEqual, "frame")
	test.That(t, spatialmath.PoseAlmostEqual(pF.Pose(), pose), test.ShouldBeTrue)
	convertedPF := ProtobufToPoseInFrame(PoseInFrameToProtobuf(pF))
	test.That(t, pF.Parent(), test.ShouldEqual, convertedPF.Parent())
	test.That(t, spatialmath.PoseAlmostEqual(pF.Pose(), convertedPF.Pose()), test.ShouldBeTrue)
}

func TestPoseInFrameWithGoalCloud(t *testing.T) {
	pose := spatialmath.NewPose(r3.Vector{1, 2, 3}, &spatialmath.OrientationVector{math.Pi / 2, 0, 0, -1})
	cloud := &PoseCloud{X: 1, Y: 2, Z: 3, OX: 0.1, OY: 0.2, OZ: 0.3, Theta: 15}
	pF := NewPoseInFrameWithGoalCloud("frame", pose, cloud)

	convertedPF := ProtobufToPoseInFrame(PoseInFrameToProtobuf(pF))
	test.That(t, convertedPF.Parent(), test.ShouldEqual, "frame")
	test.That(t, spatialmath.PoseAlmostEqual(convertedPF.Pose(), pose), test.ShouldBeTrue)
	test.That(t, convertedPF.GoalCloud, test.ShouldNotBeNil)

	test.That(t, convertedPF.GoalCloud.X, test.ShouldEqual, cloud.X)
	test.That(t, convertedPF.GoalCloud.Y, test.ShouldEqual, cloud.Y)
	test.That(t, convertedPF.GoalCloud.Z, test.ShouldEqual, cloud.Z)
	test.That(t, convertedPF.GoalCloud.OX, test.ShouldEqual, cloud.OX)
	test.That(t, convertedPF.GoalCloud.OY, test.ShouldEqual, cloud.OY)
	test.That(t, convertedPF.GoalCloud.OZ, test.ShouldEqual, cloud.OZ)
	test.That(t, convertedPF.GoalCloud.Theta, test.ShouldEqual, cloud.Theta)
}

func TestPoseInCloudObeysEveryLimit(t *testing.T) {
	// The goal is always the zero pose. Each subtest declares one cloud with a single nonzero
	// leeway — the field under test — and checks that a candidate offset by a small amount along
	// that field is admitted, while a large offset is rejected. Every other cloud leeway is left
	// at zero (admitting only candidates that match the goal in that dimension within epsilon).
	goalPose := spatialmath.NewZeroPose()

	t.Run("X", func(t *testing.T) {
		cloud := PoseCloud{X: 10}
		smallChange := spatialmath.NewPoseFromPoint(r3.Vector{X: 5})
		largeChange := spatialmath.NewPoseFromPoint(r3.Vector{X: 20})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("Y", func(t *testing.T) {
		cloud := PoseCloud{Y: 10}
		smallChange := spatialmath.NewPoseFromPoint(r3.Vector{Y: 5})
		largeChange := spatialmath.NewPoseFromPoint(r3.Vector{Y: 20})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("Z", func(t *testing.T) {
		cloud := PoseCloud{Z: 10}
		smallChange := spatialmath.NewPoseFromPoint(r3.Vector{Z: 5})
		largeChange := spatialmath.NewPoseFromPoint(r3.Vector{Z: 20})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("OX", func(t *testing.T) {
		// Orientation vectors are unit-length, so changing OX implicitly drops OZ off of 1.
		cloud := PoseCloud{OX: 0.1, OZ: 0.1}
		smallChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0.1, OY: 0, OZ: 1, Theta: 0})
		largeChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0.5, OY: 0, OZ: 1, Theta: 0})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("OY", func(t *testing.T) {
		cloud := PoseCloud{OY: 0.1, OZ: 0.1}
		smallChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0.1, OZ: 1, Theta: 0})
		largeChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0.5, OZ: 1, Theta: 0})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("OZ", func(t *testing.T) {
		// Orientation vectors are unit-length with a default of OZ: 1 (i.e: `PoseBetween` with the
		// same values gives an OZ: 1 Pose). So changing OZ implies the vector has moved some in
		// either the OX or OY direction. To test that a cloud around OZ is being measured, we let
		// OX and OY be anything.
		cloud := PoseCloud{OX: 1, OY: 1, OZ: 0.3}
		smallChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0.1, OY: 0.1, OZ: 0.8, Theta: 0})
		largeChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0.5, OY: 0.5, OZ: 0.5, Theta: 0})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})

	t.Run("Theta", func(t *testing.T) {
		cloud := PoseCloud{Theta: 15}
		smallChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1, Theta: 10})
		largeChange := spatialmath.NewPose(r3.Vector{},
			&spatialmath.OrientationVectorDegrees{OX: 0, OY: 0, OZ: 1, Theta: 20})

		test.That(t, cloud.PoseInCloud(goalPose, smallChange), test.ShouldBeTrue)
		test.That(t, cloud.PoseInCloud(goalPose, largeChange), test.ShouldBeFalse)
	})
}

func TestGeometriesInFrame(t *testing.T) {
	pose := spatialmath.NewPose(r3.Vector{1, 2, 3}, &spatialmath.OrientationVector{math.Pi / 2, 0, 0, -1})
	zero, err := spatialmath.NewBox(pose, r3.Vector{1, 2, 3}, "zero")
	test.That(t, err, test.ShouldBeNil)
	one, err := spatialmath.NewBox(pose, r3.Vector{2, 3, 4}, "one")
	test.That(t, err, test.ShouldBeNil)
	two, err := spatialmath.NewBox(pose, r3.Vector{3, 4, 5}, "two")
	test.That(t, err, test.ShouldBeNil)
	three, err := spatialmath.NewBox(pose, r3.Vector{4, 5, 6}, "three")
	test.That(t, err, test.ShouldBeNil)
	geometryList := []spatialmath.Geometry{zero, one, two, three}

	test.That(t, err, test.ShouldBeNil)
	gF := NewGeometriesInFrame("frame", geometryList)
	test.That(t, gF.Parent(), test.ShouldEqual, "frame")
	test.That(t, spatialmath.GeometriesAlmostEqual(one, gF.GeometryByName("one")), test.ShouldBeTrue)
	convertedGF, err := ProtobufToGeometriesInFrame(GeometriesInFrameToProtobuf(gF))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gF.Parent(), test.ShouldEqual, convertedGF.Parent())
	test.That(t, spatialmath.GeometriesAlmostEqual(one, convertedGF.GeometryByName("one")), test.ShouldBeTrue)
}

func TestGeometriesInFrameJSON(t *testing.T) {
	pose := spatialmath.NewPose(r3.Vector{1, 2, 3}, &spatialmath.OrientationVector{math.Pi / 2, 0, 0, -1})
	zero, err := spatialmath.NewBox(pose, r3.Vector{1, 2, 3}, "zero")
	test.That(t, err, test.ShouldBeNil)
	one, err := spatialmath.NewBox(pose, r3.Vector{2, 3, 4}, "one")
	test.That(t, err, test.ShouldBeNil)
	two, err := spatialmath.NewBox(pose, r3.Vector{3, 4, 5}, "two")
	test.That(t, err, test.ShouldBeNil)
	three, err := spatialmath.NewBox(pose, r3.Vector{4, 5, 6}, "three")
	test.That(t, err, test.ShouldBeNil)

	gF := NewGeometriesInFrame("frame", []spatialmath.Geometry{zero, one, two, three})

	data, err := json.Marshal(gF)
	test.That(t, err, test.ShouldBeNil)

	var roundTripped GeometriesInFrame
	err = json.Unmarshal(data, &roundTripped)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, roundTripped.Parent(), test.ShouldEqual, gF.Parent())
	for _, name := range []string{"zero", "one", "two", "three"} {
		test.That(t, spatialmath.GeometriesAlmostEqual(gF.GeometryByName(name), roundTripped.GeometryByName(name)), test.ShouldBeTrue)
	}
}
