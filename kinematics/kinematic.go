// Package kinematics implements various kinematics methods for use with
// robotic parts needing to describe motion.
package kinematics

import (
	"log"
	"math"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/spatialmath"

	"github.com/go-gl/mathgl/mgl64"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

// ForwardPosition will update the model state to have the correct 6d position given its joint angles
func (m *Model) ForwardPosition() {
	for _, element := range m.Elements {
		element.ForwardPosition()
	}
}

// ForwardVelocity will update the model state to have the correct velocity state
func (m *Model) ForwardVelocity() {
	for _, element := range m.Elements {
		element.ForwardVelocity()
	}
}

// GetOperationalPosition will return the position of the given end effector ID (usually 0) and its euler angles
func (m *Model) GetOperationalPosition(idx int) *spatialmath.DualQuaternion {
	return m.Nodes[m.Leaves[idx]].i.t
}

// GetJacobian TODO
func (m *Model) GetJacobian() *mgl64.MatMxN {
	return m.Jacobian
}

// GetJacobianInverse TODO
func (m *Model) GetJacobianInverse() *mgl64.MatMxN {
	return m.InvJacobian
}

// Get6dPosition returns the 6d pose of the requested end effector as a pb.ArmPosition
func (m *Model) Get6dPosition(idx int) *pb.ArmPosition {
	return m.GetOperationalPosition(idx).ToArmPos()
}

// GetOperationalVelocity will return the velocity quaternion of the given end effector ID (usually 0)
func (m *Model) GetOperationalVelocity(idx int) dualquat.Number {
	return m.Nodes[m.Leaves[idx]].GetVelocity()
}

// GetJointOperationalVelocity will return the velocity quaternion of the given joint
func (m *Model) GetJointOperationalVelocity(idx int) dualquat.Number {
	return m.Joints[idx].GetOperationalVelocity()
}

// CalculateJacobian calculates the Jacobian matrix for the end effector's current position
// This used to support multiple end effectors
// Removed that support when quaternions were added
// because nothing we have has multiple end effectors
// Multiple end effectors can be re-added here
func (m *Model) CalculateJacobian() {
	// TODO (pl): Update this to support R4 AA
	m.Jacobian = mgl64.NewMatrix(6, m.GetDof())

	q := dualquat.Number{}
	q.Real.Real = 1

	m.ForwardPosition()
	eePosition := m.GetOperationalPosition(0).Quat
	// Take the partial derivative of each degree of freedom
	// We want to see how much things change when each DOF changes
	for i := 0; i < m.GetDof(); i++ {
		vel := make([]float64, m.GetDof())
		vel[i] = 1
		m.SetVelocity(vel)
		m.ForwardVelocity()

		eeVelocity := m.GetOperationalVelocity(0)

		endRot := eePosition.Real
		endTrans := quat.Scale(2.0, quat.Mul(eePosition.Dual, quat.Conj(endRot)))

		// Change in XYZ position
		dEndTrans := quat.Mul(quat.Sub(quat.Scale(2.0, eeVelocity.Dual), quat.Mul(endTrans, eeVelocity.Real)), quat.Conj(endRot))

		orientDs := deriv(endRot)
		orientDx := quat.Mul(quat.Conj(orientDs[0]), eeVelocity.Real).Real
		orientDy := quat.Mul(quat.Conj(orientDs[1]), eeVelocity.Real).Real
		orientDz := quat.Mul(quat.Conj(orientDs[2]), eeVelocity.Real).Real

		jacQuat := dualquat.Number{quat.Number{0, orientDx, orientDy, orientDz}, dEndTrans}

		m.Jacobian.Set(0, i, jacQuat.Dual.Imag/2)
		m.Jacobian.Set(1, i, jacQuat.Dual.Jmag/2)
		m.Jacobian.Set(2, i, jacQuat.Dual.Kmag/2)
		m.Jacobian.Set(3, i, jacQuat.Real.Imag)
		m.Jacobian.Set(4, i, jacQuat.Real.Jmag)
		m.Jacobian.Set(5, i, jacQuat.Real.Kmag)
	}
}

// CalculateJacobianInverse TODO
func (m *Model) CalculateJacobianInverse(lambda float64, doSvd bool) {
	nr := m.Jacobian.NumRows()
	nc := m.Jacobian.NumCols()
	// gonum.mat and mgl64.MatMxN use reversed raw data schemes- loading a matrix this way results in a transposition
	denseJac := mat.NewDense(nr, nc, m.Jacobian.Raw())

	// Non-SVD is not as good. Don't use it.
	if doSvd {

		m.InvJacobian = mgl64.NewMatrix(nc, nr)
		m.InvJacobian.Zero(nc, nr)

		var svd mat.SVD
		ok := svd.Factorize(denseJac, mat.SVDFull)
		if !ok {
			// This should never happen I hope? RL doesn't have error handling on this step so we're probably good
			log.Fatal("failed to factorize matrix", denseJac)
		}
		lambdaSqr := 0.0
		svdValues := svd.Values(nil)
		wMin := svdValues[len(svdValues)-1]

		if wMin < 1.0e-9 {
			lambdaSqr = (1 - math.Pow(wMin/1.0e-9, 2)) * lambda * lambda
		}

		for j, svdVal := range svdValues {
			if svdVal == 0 {
				break
			}

			svdLambda := svdVal / (svdVal*svdVal + lambdaSqr)

			matU := &mat.Dense{}
			matV := &mat.Dense{}
			svd.UTo(matU)
			svd.VTo(matV)

			r, _ := matU.Dims()
			c, _ := matV.Dims()

			colV := mgl64.NewMatrixFromData(mat.Col(nil, j, matV), c, 1)
			colU := mgl64.NewMatrixFromData(mat.Col(nil, j, matU), 1, r)

			colV.Mul(colV, svdLambda)
			colV.MulMxN(colV, colU)
			colV = colV.Transpose(mgl64.NewMatrix(r, c))
			m.InvJacobian.Add(m.InvJacobian, colV)
			// TODO(pl): Settle on one matrix implementation rather than swapping between gonum/mat and mgl64/MatMxN
		}
	} else {
		// This implements the Dampened Least Squares algorithm, which does not work overly well.
		// Do not recommend using this- use the Singular Value Decomposition method above
		// But it's here if you need it
		n := m.GetOperationalDof() * 6
		trans := mat.NewDense(nc, nr, m.Jacobian.Raw())
		dampingMatrix := mat.NewDense(n, n, mgl64.IdentN(nil, n).Mul(nil, lambda*lambda).Raw())
		var m1, m2, m3, invJ mat.Dense
		m1.Mul(denseJac, trans)
		m2.Add(&m1, dampingMatrix)
		err := m3.Inverse(&m2)
		if err != nil {
			log.Fatal("failed to invert DLS matrix")
		}
		invJ.Mul(trans, &m3)
		rawIJ := invJ.RawMatrix()
		m.InvJacobian = mgl64.NewMatrixFromData(rawIJ.Data, rawIJ.Rows, rawIJ.Cols)
	}
}

// ZeroInlineRotation will look for joint angles that are approximately complementary (e.g. 0.5 and -0.5) and check if they
// are inline by seeing if moving both closer to zero changes the 6d position. If they appear to be inline it will set
// both to zero if they are not. This should avoid needless twists of inline joints.
// TODO(pl): Support additional end effectors
func (m *Model) ZeroInlineRotation(angles []float64) []float64 {
	epsilon := 0.0001

	newAngles := make([]float64, len(angles))
	copy(newAngles, angles)

	for i, angle1 := range angles {
		for j := i; j < len(angles); j++ {
			angle2 := angles[j]
			if mgl64.FloatEqualThreshold(angle1*-1, angle2, epsilon) {
				// These angles are complementary
				origAngles := m.GetPosition()
				origAnglesBak := m.GetPosition()
				origTransform := m.GetOperationalPosition(0).Clone()
				origAngles[i] = 0
				origAngles[j] = 0
				m.SetPosition(origAngles)
				m.ForwardPosition()
				distance := SquaredNorm(m.GetOperationalPosition(0).ToDelta(origTransform))

				// Check we did not move the end effector too much
				if distance < epsilon*epsilon {
					newAngles[i] = 0
					newAngles[j] = 0
				} else {
					m.SetPosition(origAnglesBak)
					m.ForwardPosition()
				}
			}
		}
	}
	return newAngles
}

// Step TODO
func (m *Model) Step(posvec, dpos []float64) []float64 {

	var posvec2 []float64

	// j is used to step over posvec
	j := 0
	// k is used to step over dpos
	k := 0

	for _, joint := range m.Joints {
		posvec2 = append(posvec2, joint.Step(posvec[j:j+joint.GetDofPosition()], dpos[k:k+joint.GetDof()])...)
		j += joint.GetDofPosition()
		k += joint.GetDof()
	}
	return posvec2
}

// deriv will compute D(q), the derivative of q = e^w with respect to w
// Note that for prismatic joints, this will need to be expanded to dual quaternions
func deriv(q quat.Number) []quat.Number {
	w := quat.Log(q)

	qNorm := math.Sqrt(w.Imag*w.Imag + w.Jmag*w.Jmag + w.Kmag*w.Kmag)
	// qNorm hits a singularity every 2pi
	// But if we flip the axis we get the same rotation but away from a singularity

	var quatD []quat.Number

	// qNorm is non-zero if our joint has a non-zero rotation
	if qNorm > 0 {
		b := math.Sin(qNorm) / qNorm
		c := (math.Cos(qNorm) / (qNorm * qNorm)) - (math.Sin(qNorm) / (qNorm * qNorm * qNorm))

		quatD = append(quatD, quat.Number{Real: -1 * w.Imag * b,
			Imag: b + w.Imag*w.Imag*c,
			Jmag: w.Imag * w.Jmag * c,
			Kmag: w.Imag * w.Kmag * c})
		quatD = append(quatD, quat.Number{Real: -1 * w.Jmag * b,
			Imag: w.Jmag * w.Imag * c,
			Jmag: b + w.Jmag*w.Jmag*c,
			Kmag: w.Jmag * w.Kmag * c})
		quatD = append(quatD, quat.Number{Real: -1 * w.Kmag * b,
			Imag: w.Kmag * w.Imag * c,
			Jmag: w.Kmag * w.Jmag * c,
			Kmag: b + w.Kmag*w.Kmag*c})

	} else {
		quatD = append(quatD, quat.Number{0, 1, 0, 0})
		quatD = append(quatD, quat.Number{0, 0, 1, 0})
		quatD = append(quatD, quat.Number{0, 0, 0, 1})
	}
	return quatD
}
