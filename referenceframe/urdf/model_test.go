package urdf

import (
	"encoding/xml"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestParseURDFFile(t *testing.T) {
	// Test a URDF which has prismatic joints
	u, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/example_gantry.xml"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 2)

	// Test a URDF will has collision geometries we can evaluate and a DoF of 6
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/ur5e.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	model, ok := u.(*referenceframe.SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, u.Name(), test.ShouldEqual, "ur5")
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)
	modelGeo, err := model.Geometries(make([]referenceframe.Input, len(model.DoF())))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(modelGeo.Geometries()), test.ShouldEqual, 5) // notably we only have 5 geometries for this model

	// Test naming of a URDF to something other than the robot's name element
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/ur5e.urdf"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "foo")
}

func TestURDFTransforms(t *testing.T) {
	u, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/urdf/testfiles/ur5e.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	simple, ok := u.(*referenceframe.SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	joints := []referenceframe.Frame{}
	for _, tform := range simple.OrdTransforms {
		if len(tform.DoF()) > 0 {
			joints = append(joints, tform)
		}
	}
	test.That(t, len(joints), test.ShouldEqual, 6)
	pose, err := joints[0].Transform([]referenceframe.Input{{0}})
	test.That(t, err, test.ShouldBeNil)
	firstJov := pose.Orientation().OrientationVectorRadians()
	firstJovExpect := &spatialmath.OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov, test.ShouldResemble, firstJovExpect)

	pose, err = joints[0].Transform([]referenceframe.Input{{1.5708}})
	test.That(t, err, test.ShouldBeNil)
	firstJov = pose.Orientation().OrientationVectorRadians()
	firstJovExpect = &spatialmath.OrientationVector{Theta: 1.5708, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov.Theta, test.ShouldAlmostEqual, firstJovExpect.Theta)
	test.That(t, firstJov.OX, test.ShouldAlmostEqual, firstJovExpect.OX)
	test.That(t, firstJov.OY, test.ShouldAlmostEqual, firstJovExpect.OY)
	test.That(t, firstJov.OZ, test.ShouldAlmostEqual, firstJovExpect.OZ)
}

func TestWorldStateConversion(t *testing.T) {
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 1, Y: 2, Z: 3}, "bar")
	test.That(t, err, test.ShouldBeNil)
	ws, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, []spatialmath.Geometry{foo, bar})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	cfg, err := NewModelFromWorldState(ws, "test")
	test.That(t, err, test.ShouldBeNil)
	bytes, err := xml.MarshalIndent(cfg, "", "  ")
	test.That(t, err, test.ShouldBeNil)
	_ = bytes
}
