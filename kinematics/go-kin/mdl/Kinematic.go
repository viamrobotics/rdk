package mdl

import (
	//~ "fmt"
	"log"
	"math"

	"github.com/go-gl/mathgl/mgl64"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath"
	"github.com/viamrobotics/robotcore/kinematics/go-kin/kinmath/spatial"
	"gonum.org/v1/gonum/mat"
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
func (m *Model) GetOperationalPosition(idx int) *kinmath.Transform {
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

	mat := m.GetOperationalPosition(idx).Matrix()
	// Get xyz position
	pose6d = append(pose6d, mat.At(0, 3))
	pose6d = append(pose6d, mat.At(1, 3))
	pose6d = append(pose6d, mat.At(2, 3))

	// Get euler angles
	pose6d = append(pose6d, MatToEuler(mat)...)
	return pose6d
}

// GetOperationalVelocity will return the velocity vector of the given end effector ID (usually 0)
func (m *Model) GetOperationalVelocity(idx int) spatial.MotionVector {
	return m.Nodes[m.Leaves[idx]].GetVelocityVector()
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

func (m *Model) CalculateJacobian() {
	inWorldFrame := true

	m.Jacobian = mgl64.NewMatrix(m.GetOperationalDof()*6, m.GetDof())

	for i := 0; i < m.GetDof(); i++ {
		fakeVel := SetZero(m.GetDof())
		for j := 0; j < m.GetDof(); j++ {
			if i == j {
				fakeVel[j] = 1
			}
		}
		m.SetVelocity(fakeVel)
		m.ForwardVelocity()

		for j := 0; j < m.GetOperationalDof(); j++ {
			if inWorldFrame {
				j1 := m.GetOperationalPosition(j).Linear().Mul3x1(m.GetOperationalVelocity(j).Linear)
				m.Jacobian.Set(j*6, i, j1.X())
				m.Jacobian.Set(j*6+1, i, j1.Y())
				m.Jacobian.Set(j*6+2, i, j1.Z())
				j2 := m.GetOperationalPosition(j).Linear().Mul3x1(m.GetOperationalVelocity(j).Angular)
				m.Jacobian.Set(j*6+3, i, j2.X())
				m.Jacobian.Set(j*6+4, i, j2.Y())
				m.Jacobian.Set(j*6+5, i, j2.Z())
			}
		}
	}
}

func (m *Model) CalculateJacobianInverse(lambda float64, doSvd bool) {

	if doSvd {
		nr := m.Jacobian.NumRows()
		nc := m.Jacobian.NumCols()

		m.InvJacobian = mgl64.NewMatrix(nr, nc)
		m.InvJacobian.Zero(nr, nc)

		svdMat := mat.NewDense(nr, nc, m.Jacobian.Raw())
		var svd mat.SVD
		ok := svd.Factorize(svdMat, mat.SVDFull)
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

			colV := mgl64.NewMatrixFromData(mat.Col(nil, j, matV), r, 1)
			colU := mgl64.NewMatrixFromData(mat.Col(nil, j, matU), 1, r)

			colV.Mul(colV, svdLambda)
			colV.MulMxN(colV, colU)
			colV = colV.Transpose(mgl64.NewMatrix(r, r))
			m.InvJacobian.Add(m.InvJacobian, colV)
			// TODO: Settle on one matrix implementation rather than swapping between gonum/mat and mgl64/MatMxN
		}

	} else {
		// Not done, do not use. Missing matrix inversion, etc
		//~ m.Jacobian.Transpose().MulMxN(nil, m.Jacobian.Transpose().MulMxN(nil, m.Jacobian).Add(nil, mgl64.IdentN(nil, m.GetOperationalDof() * 6).Mul(nil, lambda * lambda)))
	}
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
