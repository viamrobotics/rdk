package referenceframe

import (
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestParseURDFFile(t *testing.T) {
	// Test a URDF which has prismatic joints
	u, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/example_gantry.xml"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 2)

	// Test a URDF will has collision geometries we can evaluate and a DoF of 6
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	model, ok := u.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, u.Name(), test.ShouldEqual, "ur5")
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)
	modelGeo, err := model.Geometries(make([]Input, len(model.DoF())))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(modelGeo.Geometries()), test.ShouldEqual, 5) // notably we only have 5 geometries for this model

	// Test naming of a URDF to something other than the robot's name element
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur5e.urdf"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "foo")
}

func TestWorldStateConversion(t *testing.T) {
	foo, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 10, "foo")
	test.That(t, err, test.ShouldBeNil)
	bar, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{X: 1, Y: 2, Z: 3}, "bar")
	test.That(t, err, test.ShouldBeNil)
	ws, err := NewWorldState(
		[]*GeometriesInFrame{NewGeometriesInFrame(World, []spatialmath.Geometry{foo, bar})},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	cfg, err := NewModelFromWorldState(ws, "test")
	test.That(t, err, test.ShouldBeNil)
	bytes, err := xml.MarshalIndent(cfg, "", "  ")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bytes, test.ShouldNotBeNil)
}

func TestURDFWithMeshes(t *testing.T) {
	// ufactory 850
	u, err := ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/uf850.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)

	p, _ := u.Transform([]Input{0, 0, 0, 0, 0, 0})
	fmt.Println(p)

	// ur20
	u, err = ParseModelXMLFile(utils.ResolveFile("referenceframe/testfiles/ur20.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)

	p, _ = u.Transform([]Input{0, 0, 0, 0, 0, 0})
	fmt.Println(p)
}
