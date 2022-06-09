package odometry

import (
	"testing"

	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
)

func TestNewMotion3DFromRotationTranslation(t *testing.T) {
	// rotation = Id
	rot := mat.NewDense(3, 3, nil)
	rot.Set(0, 0, 1)
	rot.Set(1, 1, 1)
	rot.Set(2, 2, 1)

	// Translation = 1m in z direction
	tr := mat.NewDense(3, 1, []float64{0, 0, 1})

	motion := NewMotion3DFromRotationTranslation(rot, tr)
	test.That(t, motion, test.ShouldNotBeNil)
	test.That(t, motion.Rotation, test.ShouldResemble, rot)
	test.That(t, motion.Translation, test.ShouldResemble, tr)
}
