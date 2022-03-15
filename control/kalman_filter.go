package control

import "gonum.org/v1/gonum/mat"

//

type kalmanFilter struct {
	state      *mat.VecDense // System State Matrix
	covariance *mat.Dense    // Covariance Matrix
}

type model struct {
}

// input model/ current state/
func (kF *kalmanFilter) NextCovariance() error {
	//has to be square
	kF.covariance = mat.NewDense(1, 1, []float64{1})
	return nil
}

func (kF *kalmanFilter) NextState() (mat.Matrix, error) {
	kF.state = mat.NewVecDense(1, []float64{1})
	return kF.covariance, nil
}

func (kF *kalmanFilter) Predict() error {
	return nil
}
func (kF *kalmanFilter) Update() error {
	return nil
}

func (kF *kalmanFilter) State() (mat.Vector, error) {
	return kF.state, nil
}
