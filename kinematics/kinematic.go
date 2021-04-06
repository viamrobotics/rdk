package kinematics

import (
	//~ "fmt"
	"log"
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"go.viam.com/robotcore/kinematics/kinmath"

	//~ "go.viam.com/robotcore/kinematics/kinmath/spatial"
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
func (m *Model) GetOperationalPosition(idx int) *kinmath.QuatTrans {
	return m.Nodes[m.Leaves[idx]].i.t
}

func (m *Model) GetJacobian() *mgl64.MatMxN {
	return m.Jacobian
}

func (m *Model) GetJacobianInverse() *mgl64.MatMxN {
	return m.InvJacobian
}

func (m *Model) Get6dPosition(idx int) []float64 {
	var pose6d []float64

	endTransform := m.GetOperationalPosition(idx)
	quat := endTransform.Quat
	cartQuat := dualquat.Mul(quat, dualquat.Conj(quat))
	// Get xyz position
	pose6d = append(pose6d, cartQuat.Dual.Imag)
	pose6d = append(pose6d, cartQuat.Dual.Jmag)
	pose6d = append(pose6d, cartQuat.Dual.Kmag)

	// Get euler angles
	pose6d = append(pose6d, QuatToEuler(quat.Real)...)
	return pose6d
}

// GetOperationalVelocity will return the velocity quaternion of the given end effector ID (usually 0)
func (m *Model) GetOperationalVelocity(idx int) dualquat.Number {
	return m.Nodes[m.Leaves[idx]].GetVelocity()
}

// GetJointOperationalVelocity will return the velocity quaternion of the given joint
func (m *Model) GetJointOperationalVelocity(idx int) dualquat.Number {
	return m.Joints[idx].GetOperationalVelocity()
}

// Bit of a weird thing we use this for
// The quat in the transform actually describes the axis of an axis angle
// So we want to get the direction the axis is pointing in
func QuatToEuler(q quat.Number) []float64 {
	w := q.Real
	x := q.Imag
	y := q.Jmag
	z := q.Kmag

	var angles []float64

	angles = append(angles, math.Atan2(2*(w*x+y*z), 1-2*(x*x+y*y)))
	angles = append(angles, math.Asin(2*(w*y-x*z)))
	angles = append(angles, math.Atan2(2*(w*z+y*x), 1-2*(y*y+z*z)))

	for i := range angles {

		angles[i] *= 180 / math.Pi
	}
	return angles
}

func MatToEuler(mat mgl64.Mat4) []float64 {
	sy := math.Sqrt(mat.At(0, 0)*mat.At(0, 0) + mat.At(1, 0)*mat.At(1, 0))
	singular := sy < 1e-6
	var angles []float64
	if !singular {
		angles = append(angles, math.Atan2(mat.At(2, 1), mat.At(2, 2)))
		angles = append(angles, math.Atan2(-mat.At(2, 0), sy))
		angles = append(angles, math.Atan2(mat.At(1, 0), mat.At(0, 0)))
	} else {
		angles = append(angles, math.Atan2(-mat.At(1, 2), mat.At(1, 1)))
		angles = append(angles, math.Atan2(-mat.At(2, 0), sy))
		angles = append(angles, 0)
	}
	for i := range angles {
		angles[i] *= 180 / math.Pi
	}
	return angles
}

// This used to support multiple end effectors
// Removed that support when quaternions were added
// because nothing we have has multiple end effectors, and I didn't need to worry about it
// Multiple end effectors can be re-added here
func (m *Model) CalculateJacobian() {
	//~ inWorldFrame := true

	m.Jacobian = mgl64.NewMatrix(6, m.GetDof())

	q := dualquat.Number{}
	q.Real.Real = 1

	m.ForwardPosition()
	EEPosition := m.GetOperationalPosition(0).Quat
	// Take the partial derivative of each degree of freedom
	// We want to see how much things change when each DOF changes
	for i := 0; i < m.GetDof(); i++ {
		vel := make([]float64, m.GetDof())
		vel[i] = 1
		m.SetVelocity(vel)
		m.ForwardVelocity()

		EEVelocity := m.GetOperationalVelocity(0)

		endRot := EEPosition.Real
		endTrans := quat.Scale(2.0, quat.Mul(EEPosition.Dual, quat.Conj(endRot)))

		// Change in XYZ position
		dEndTrans := quat.Mul(quat.Sub(quat.Scale(2.0, EEVelocity.Dual), quat.Mul(endTrans, EEVelocity.Real)), quat.Conj(endRot))

		orientDs := deriv(endRot)
		orientDx := quat.Mul(quat.Conj(orientDs[0]), EEVelocity.Real).Real
		orientDy := quat.Mul(quat.Conj(orientDs[1]), EEVelocity.Real).Real
		orientDz := quat.Mul(quat.Conj(orientDs[2]), EEVelocity.Real).Real

		jacQuat := dualquat.Number{quat.Number{0, orientDx, orientDy, orientDz}, dEndTrans}

		//~ m.Jacobian.Set(0, i, jacQuat.Real.Real)
		m.Jacobian.Set(3, i, jacQuat.Real.Imag)
		m.Jacobian.Set(4, i, jacQuat.Real.Jmag)
		m.Jacobian.Set(5, i, jacQuat.Real.Kmag)
		//~ m.Jacobian.Set(4, i, jacQuat.Dual.Real)
		m.Jacobian.Set(0, i, jacQuat.Dual.Imag/2)
		m.Jacobian.Set(1, i, jacQuat.Dual.Jmag/2)
		m.Jacobian.Set(2, i, jacQuat.Dual.Kmag/2)

		//~ for j := 0; j < m.GetOperationalDof(); j++ {
		//~ if inWorldFrame {
		//~ j1 := m.GetOperationalPosition(j).Rotation().Mul3x1(m.GetOperationalVelocity(j).Linear)
		//~ m.Jacobian.Set(j*6, i, j1.X())
		//~ m.Jacobian.Set(j*6+1, i, j1.Y())
		//~ m.Jacobian.Set(j*6+2, i, j1.Z())
		//~ j2 := m.GetOperationalPosition(j).Rotation().Mul3x1(m.GetOperationalVelocity(j).Angular)
		//~ m.Jacobian.Set(j*6+3, i, j2.X())
		//~ m.Jacobian.Set(j*6+4, i, j2.Y())
		//~ m.Jacobian.Set(j*6+5, i, j2.Z())
		//~ }
		//~ }
	}
}

func (m *Model) CalculateJacobianInverse(lambda float64, doSvd bool) {
	nr := m.Jacobian.NumRows()
	nc := m.Jacobian.NumCols()
	// gonum.mat and mgl64.MatMxN use reversed raw data schemes
	denseJac := mat.NewDense(nr, nc, m.Jacobian.Raw())

	// Non-SVD is not as good. Don't use it.
	if doSvd {

		m.InvJacobian = mgl64.NewMatrix(nc, nr)
		m.InvJacobian.Zero(nc, nr)

		var svd mat.SVD
		ok := svd.Factorize(denseJac, mat.SVDFull)
		if !ok {
			// This should never happen I hope? RL doesn't have error handling on this step so we're probably good
			log.Fatal("failed to factorize matrix")
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

			//~ fmt.Println("r", r)
			//~ fmt.Println("c", c)

			colV := mgl64.NewMatrixFromData(mat.Col(nil, j, matV), c, 1)
			//~ fmt.Println("colV", colV)
			colU := mgl64.NewMatrixFromData(mat.Col(nil, j, matU), 1, r)
			//~ fmt.Println("colU", colU)

			colV.Mul(colV, svdLambda)
			colV.MulMxN(colV, colU)
			//~ fmt.Println("colV-T", colV)
			colV = colV.Transpose(mgl64.NewMatrix(r, c))
			//~ fmt.Println("colV-add", colV)
			m.InvJacobian.Add(m.InvJacobian, colV)
			// TODO(pl): Settle on one matrix implementation rather than swapping between gonum/mat and mgl64/MatMxN
		}

		//~ } else {
		//~ n := m.GetOperationalDof() * 6
		//~ trans := mat.NewDense(nc, nr, m.Jacobian.Raw())
		//~ dampingMatrix := mat.NewDense(n, n, mgl64.IdentN(nil, n).Mul(nil, lambda*lambda).Raw())
		//~ var m1, m2, m3, invJ mat.Dense
		//~ m1.Mul(denseJac, trans)
		//~ m2.Add(&m1, dampingMatrix)
		//~ m3.Inverse(&m2)
		//~ invJ.Mul(trans, &m3)
		//~ rawIJ := invJ.RawMatrix()

		//~ m.InvJacobian = mgl64.NewMatrixFromData(rawIJ.Data, rawIJ.Rows, rawIJ.Cols)
	}
}

// This function will look for joint angles that are approximately complementary (e.g. 0.5 and -0.5) and check if they
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
