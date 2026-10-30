package kinematics_test

import (
	"strings"
	"testing"

	"go.viam.com/test"

	models3d "go.viam.com/rdk/components/arm/fake/3d_models"
	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/referenceframe"
)

const testArmName = "arm"

// The 3D scene draws one node per geometry an arm reports and swaps in the 3D model whose name
// matches that geometry's label. A link with a visual but no geometry to hang it on is never
// drawn, so every link that ships a visual needs a geometry on it.
func TestEveryModelPartHasAGeometry(t *testing.T) {
	for _, armModel := range []string{"ur5e", "ur20", "xarm6", "lite6"} {
		t.Run(armModel, func(t *testing.T) {
			visuals := models3d.Visuals(armModel)
			test.That(t, visuals, test.ShouldNotBeEmpty)
			model, err := kinematics.ModelFromName(armModel, testArmName)
			test.That(t, err, test.ShouldBeNil)

			gif, err := model.Geometries(make([]referenceframe.Input, len(model.DoF())))
			test.That(t, err, test.ShouldBeNil)

			labels := make(map[string]bool)
			for _, geometry := range gif.Geometries() {
				labels[strings.TrimPrefix(geometry.Label(), testArmName+":")] = true
			}

			for link := range visuals {
				test.That(t, labels[link], test.ShouldBeTrue)
			}
		})
	}
}

func TestModelFromName(t *testing.T) {
	model, err := kinematics.ModelFromName(kinematics.XArm6, testArmName)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.Name(), test.ShouldEqual, testArmName)
	test.That(t, len(model.DoF()), test.ShouldEqual, 6)

	_, err = kinematics.ModelFromName("not-an-arm", testArmName)
	test.That(t, err, test.ShouldNotBeNil)
}
