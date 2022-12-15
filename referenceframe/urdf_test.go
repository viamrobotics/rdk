package referenceframe

import (
	"math/rand"
	"testing"

	"go.viam.com/test"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestParseURDFFile(t *testing.T) {
	u, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5_minimal.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	simple, ok := u.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, u.Name(), test.ShouldEqual, "ur5")
	test.That(t, len(u.DoF()), test.ShouldEqual, 6)

	err = simple.validInputs(FloatsToInputs([]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}))
	test.That(t, err, test.ShouldBeNil)

	randpos := GenerateRandomConfiguration(u, rand.New(rand.NewSource(1)))
	test.That(t, simple.validInputs(FloatsToInputs(randpos)), test.ShouldBeNil)

	// Test a URDF which has prismatic joints
	u, err = ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/example_gantry.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(u.DoF()), test.ShouldEqual, 2)

	// Test naming of a URDF to something other than the robot's name element
	u, err = ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5_minimal.urdf"), "foo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "foo")
}

//nolint:dupl
func TestURDFTransforms(t *testing.T) {
	u, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5_minimal.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	simple, ok := u.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	joints := []Frame{}
	for _, tform := range simple.OrdTransforms {
		if len(tform.DoF()) > 0 {
			joints = append(joints, tform)
		}
	}
	test.That(t, len(joints), test.ShouldEqual, 6)
	pose, err := joints[0].Transform([]Input{{0}})
	test.That(t, err, test.ShouldBeNil)
	firstJov := pose.Orientation().OrientationVectorRadians()
	firstJovExpect := &spatial.OrientationVector{Theta: 0, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov, test.ShouldResemble, firstJovExpect)

	pose, err = joints[0].Transform([]Input{{1.5708}})
	test.That(t, err, test.ShouldBeNil)
	firstJov = pose.Orientation().OrientationVectorRadians()
	firstJovExpect = &spatial.OrientationVector{Theta: 1.5708, OX: 0, OY: 0, OZ: 1}
	test.That(t, firstJov.Theta, test.ShouldAlmostEqual, firstJovExpect.Theta)
	test.That(t, firstJov.OX, test.ShouldAlmostEqual, firstJovExpect.OX)
	test.That(t, firstJov.OY, test.ShouldAlmostEqual, firstJovExpect.OY)
	test.That(t, firstJov.OZ, test.ShouldAlmostEqual, firstJovExpect.OZ)
}

func TestURDFGeometries(t *testing.T) {
	ur5Min, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5_minimal.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	ur5MinModel, ok := ur5Min.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	inputs := make([]Input, len(ur5MinModel.DoF()))
	modelGeo, _ := ur5MinModel.Geometries(inputs)
	test.That(t, len(modelGeo.geometries), test.ShouldEqual, 0)

	// ur5 (Minimal) has no collision objects, but ur5 (Viam) will have collision geometries we can evaluate
	ur5Viam, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5_viam.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	ur5ViamModel, ok := ur5Viam.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	inputs = make([]Input, len(ur5ViamModel.DoF()))
	modelGeo, _ = ur5ViamModel.Geometries(inputs)
	test.That(t, len(modelGeo.geometries), test.ShouldEqual, 5)
}
