package referenceframe

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/utils"
)

func TestConvertURDF(t *testing.T) {
	u, err := ParseURDFFile(utils.ResolveFile("referenceframe/testurdf/ur5.urdf"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, u.Name(), test.ShouldEqual, "ur5")

	simpleM, ok := u.(*SimpleModel)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(u.DoF()), test.ShouldEqual, 6)

	err = simpleM.validInputs(FloatsToInputs([]float64{0.0, 0.0, 0.0, 0.0, 0.0, 0.0}))
	test.That(t, err, test.ShouldBeNil)
	err = simpleM.validInputs(FloatsToInputs([]float64{100.0, 0.0, 0.0, 0.0, 0.0, 0.0}))
	test.That(t, err, test.ShouldNotBeNil)

	// TODO(wspies): Finish out writing tests
}
