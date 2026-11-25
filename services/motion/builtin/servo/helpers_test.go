package servo

import (
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/quat"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/arm/kinematics"
	"go.viam.com/rdk/referenceframe"
)

// loadModel returns a test arm model: "ur5e" or "xarm6" from the kinematics registry, or "so101"
// (5-DOF, testdata/so101.urdf with meshes stripped).
func loadModel(t testing.TB, name string) referenceframe.Model {
	t.Helper()
	if name == "so101" {
		m, err := referenceframe.ParseModelXMLFile("testdata/so101.urdf", "arm", nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(m.DoF()), test.ShouldEqual, 5)
		return m
	}
	m, err := kinematics.ModelFromName(name, "arm")
	test.That(t, err, test.ShouldBeNil)
	return m
}

// rotZ returns a quaternion for a rotation of a radians about base z.
func rotZ(a float64) quat.Number {
	return quat.Number{Real: math.Cos(a / 2), Kmag: math.Sin(a / 2)}
}

// rotX returns a quaternion for a rotation of a radians about base x.
func rotX(a float64) quat.Number {
	return quat.Number{Real: math.Cos(a / 2), Imag: math.Sin(a / 2)}
}

// relErr is ‖a − b‖ / ‖a‖.
func relErr(a, b r3.Vector) float64 { return a.Sub(b).Norm() / a.Norm() }

//nolint:gocritic // matrix notation
func mulVec(A mat.Matrix, x []float64) []float64 {
	var y mat.VecDense
	y.MulVec(A, mat.NewVecDense(len(x), x))
	return y.RawVector().Data
}

func nearVec(t *testing.T, got, want []float64, tol float64) {
	t.Helper()
	for i := range want {
		if math.Abs(got[i]-want[i]) > tol {
			t.Fatalf("got %v, want %v (tol %g)", got, want, tol)
		}
	}
}

func vnear(t *testing.T, got, want r3.Vector, tol float64) {
	t.Helper()
	if got.Sub(want).Norm() > tol {
		t.Fatalf("got %v, want %v", got, want)
	}
}

var (
	so101Qa = []float64{0, 0.3, 0.3, 0.5, 0}
	so101Qb = []float64{0.3, 0.5, -0.8, 0.4, 0.7}
	so101Qc = []float64{0, 0.6, 0.6, 1.0, 0}
)

func mustFK(t *testing.T, m referenceframe.Model, q []float64) (r3.Vector, quat.Number) {
	t.Helper()
	p, o, err := FK(m, q)
	test.That(t, err, test.ShouldBeNil)
	return p, o
}

func mustJacobian(t *testing.T, m referenceframe.Model, q []float64) *mat.Dense {
	t.Helper()
	J, err := Jacobian(m, q)
	test.That(t, err, test.ShouldBeNil)
	return J
}

// testLimits returns the model's position limits with velocity 1 rad/s and acceleration 4 rad/s² for every
// joint. The UR5e and xArm6 JSON models and RDK's URDF parser carry no velocity/acceleration limits, so
// NewLimits needs them from MoveOptions. Tasks 5–8 reuse this helper.
func testLimits(t testing.TB, m referenceframe.Model) Limits {
	t.Helper()
	l, err := NewLimits(m.DoF(), arm.MoveOptions{MaxVelRads: 1, MaxAccRads: 4}, 90)
	test.That(t, err, test.ShouldBeNil)
	return l
}
