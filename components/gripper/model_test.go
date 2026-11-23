package gripper_test

import (
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// A model made from geometries alone has to come out as a typed model carrying them, since that
// is what a client reads first, and a gripper with no geometry serves nothing rather than failing.
func TestMakeModelTypedModel(t *testing.T) {
	jaw, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{Z: 10}), r3.Vector{X: 20, Y: 20, Z: 20}, "jaw")
	test.That(t, err, test.ShouldBeNil)
	palm, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 40, Y: 40, Z: 5}, "palm")
	test.That(t, err, test.ShouldBeNil)

	model, err := gripper.MakeModel("hand", []spatialmath.Geometry{palm, jaw})
	test.That(t, err, test.ShouldBeNil)
	pb, err := referenceframe.ModelToProto(model)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pb.GetName(), test.ShouldEqual, "hand")
	test.That(t, pb.GetJoints(), test.ShouldBeEmpty)
	test.That(t, pb.GetLinks(), test.ShouldHaveLength, 2)
	var labels []string
	for _, link := range pb.GetLinks() {
		test.That(t, link.GetCollision(), test.ShouldHaveLength, 1)
		test.That(t, link.GetCollision()[0].GetBox(), test.ShouldNotBeNil)
		labels = append(labels, link.GetCollision()[0].GetLabel())
	}
	test.That(t, labels, test.ShouldResemble, []string{"palm", "jaw"})

	back, err := referenceframe.ModelFromProto(pb, "hand")
	test.That(t, err, test.ShouldBeNil)
	gif, err := back.Geometries(nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, gif.Geometries(), test.ShouldHaveLength, 2)

	empty, err := gripper.MakeModel("bare", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, referenceframe.KinematicModelToProtobuf(empty).GetModel(), test.ShouldBeNil)
}
