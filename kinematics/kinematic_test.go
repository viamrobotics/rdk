package kinematics

import (
	"math"
	"testing"

	"go.viam.com/core/utils"

	"go.viam.com/test"
	"gonum.org/v1/gonum/num/quat"
)

// This should test forward kinematics functions
func TestForwardKinematics(t *testing.T) {
	// Test fake 5DOF arm
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 300, 0, 360.25
	m.ForwardPosition()
	expect := []float64{300, 0, 360.25, 0, 1, 0, 0}
	actual := m.Get6dPosition(0)

	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	// Test the 6dof arm we actually have
	m, err = ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	m.ForwardPosition()
	expect = []float64{365, 0, 360.25, 0, 1, 0, 0}
	actual = m.Get6dPosition(0)

	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.00001)

	newPos := []float64{0.7854, -0.7854, 0, 0, 0, 0}
	m.SetPosition(newPos)
	m.ForwardPosition()
	actual = m.Get6dPosition(0)

	expect = []float64{57.5, 57.5, 545.1208197765168, 1.096, 0.28108, -0.6786, 0.6786}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)
	newPos = []float64{-0.7854, 0, 0, 0, 0, 0.7854}
	m.SetPosition(newPos)
	m.ForwardPosition()
	actual = m.Get6dPosition(0)

	expect = []float64{258.0935, -258.0935, 360.25, 1.096, 0.6786, -0.28108, -0.6786}
	test.That(t, floatDelta(expect, actual), test.ShouldBeLessThanOrEqualTo, 0.01)

	// Test the 6dof arm we actually have
	m, err = ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/ur5.json"))
	test.That(t, err, test.ShouldBeNil)

	// Confirm end effector starts at 365, 0, 360.25
	m.ForwardPosition()
	newPos = []float64{0, 1.5708, 1.5708, 0, 0, 0}
	m.SetPosition(newPos)
	m.ForwardPosition()
}

func floatDelta(l1, l2 []float64) float64 {
	delta := 0.0
	for i, v := range l1 {
		delta += math.Abs(v - l2[i])
	}
	return delta
}

func TestJacobian_5DOF(t *testing.T) {
	j1 := []float64{0, 300, 0, 0, 0, 1, 250, 0, -300, 0, 1, 0, 0, 0, -250, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0}
	j2 := []float64{-112.20691814553123, 72.04723359799343, -3.552713678800501e-15, -0.6728542729024067, -0.1870245270979385, 0.14245346744036697, -72.57466434750532, -113.02834286904155, -133.34615235859556, -0.188440196136817, 0.4604125568357335, -0.6422462939518608, -122.82387410847048, -191.28685030857324, 104.03670913678563, -0.188440196136817, 0.4604125568357335, -0.6422462939518608, 1.5059727577447855e-14, -1.4873437542892977e-14, -1.9288441471654887e-15, 0.6431552845883316, -0.1870245270979384, -0.24367425562605577, 2.931395015491471e-15, -2.3723263558860777e-15, -7.144119634710407e-15, 0.07619642011560578, 0.9708076900883328, -0.1870245270979385}
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s_test.json"))
	test.That(t, err, test.ShouldBeNil)
	newPos := []float64{0, 0, 0, 0, 0}
	m.SetPosition(newPos)
	m.ForwardPosition()

	m.CalculateJacobian()
	j := m.GetJacobian()

	test.That(t, floatDelta(j.Raw(), j1), test.ShouldBeLessThanOrEqualTo, 0.00001)
	// Convenient position at askew angle
	newPos = []float64{1, 1, 1, 1, 1}
	m.SetPosition(newPos)
	m.ForwardPosition()

	m.CalculateJacobian()
	j = m.GetJacobian()

	test.That(t, floatDelta(j.Raw(), j2), test.ShouldBeLessThanOrEqualTo, 0.00001)
}

func TestJacobian_6DOF(t *testing.T) {
	j1 := []float64{0, 365, 0, 0, 0, 1, 250, 0, -365, 0, 1, 0, 0, 0, -315, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, -65, 0, 1, 0, 0, 0, 0, 1, 0, 0}
	j2 := []float64{-102.16440483663916, 10.903395944418287, 0, -0.5601207763322, -0.4051355255220106, 0.4627905443295226, -83.18413050443456, -129.55160741630795, -91.85951232076194, -0.3217838668680013, -0.031119176509585317, -0.6839728452644256, -133.43334026539975, -207.81011485583966, 145.52334917461928, -0.3217838668680013, -0.031119176509585317, -0.6839728452644256, -2.255527239861759, 51.18283846598304, -19.153063348748443, 0.6256300318405804, 0.036251775629715, -0.3790931178783241, -21.891693959310256, 20.60132073450243, 57.63106210704488, -0.1915798198060357, 0.7357375249055091, -0.6083204737253887, 1.0838659909342689e-14, -1.0848734169643908e-14, -2.6105742847895922e-14, 0.11975650035482571, -0.4090529397652003, -0.5601207763321999}
	m, err := ParseJSONFile(utils.ResolveFile("kinematics/models/mdl/wx250s.json"))
	test.That(t, err, test.ShouldBeNil)
	newPos := []float64{0, 0, 0, 0, 0, 0}
	m.SetPosition(newPos)
	m.ForwardPosition()

	m.CalculateJacobian()
	j := m.GetJacobian()

	test.That(t, floatDelta(j.Raw(), j1), test.ShouldBeLessThanOrEqualTo, 0.00001)
	// Convenient position at askew angle
	newPos = []float64{1, 1, 1, 1, 1, 1}
	m.SetPosition(newPos)
	m.ForwardPosition()

	m.CalculateJacobian()
	j = m.GetJacobian()
	test.That(t, floatDelta(j.Raw(), j2), test.ShouldBeLessThanOrEqualTo, 0.00001)
}

const derivEqualityEpsilon = 1e-16

func derivComponentAlmostEqual(left, right float64) bool {
	return math.Abs(left-right) <= derivEqualityEpsilon
}

func areDerivsEqual(q1, q2 []quat.Number) bool {
	if len(q1) != len(q2) {
		return false
	}
	for i, dq1 := range q1 {
		dq2 := q2[i]
		if !derivComponentAlmostEqual(dq1.Real, dq2.Real) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Imag, dq2.Imag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Jmag, dq2.Jmag) {
			return false
		}
		if !derivComponentAlmostEqual(dq1.Kmag, dq2.Kmag) {
			return false
		}
	}
	return true
}

func TestDeriv(t *testing.T) {
	// Test identity quaternion
	q := quat.Number{1, 0, 0, 0}
	qDeriv := []quat.Number{{0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}

	match := areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity single-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 0, 0})

	qDeriv = []quat.Number{{-0.9092974268256816, -0.4161468365471424, 0, 0},
		{0, 0, 0.4546487134128408, 0},
		{0, 0, 0, 0.4546487134128408}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)

	// Test non-identity multi-axis unit quaternion
	q = quat.Exp(quat.Number{0, 2, 1.5, 0.2})

	qDeriv = []quat.Number{{-0.472134934000233, -0.42654977821280804, -0.4969629339096933, -0.06626172452129245},
		{-0.35410120050017474, -0.4969629339096933, -0.13665473343215354, -0.049696293390969336},
		{-0.0472134934000233, -0.06626172452129245, -0.049696293390969336, 0.22944129454798728}}

	match = areDerivsEqual(qDeriv, deriv(q))
	test.That(t, match, test.ShouldBeTrue)
}
