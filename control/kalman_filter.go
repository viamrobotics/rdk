package control

import "gonum.org/v1/gonum/mat"

//

type kalmanFilter struct {
	X *mat.VecDense // System State Matrix
	P *mat.Dense    // Covariance Matrix
}

// input model/ current state/
func (kF *kalmanFilter) NextCovariance() error {
	kF.P = mat.NewDense(1, 1, []float64{1})
	return nil
}
