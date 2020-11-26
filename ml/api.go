package ml

// TODO: this is all wrong, I just need a pivot point in the sand

type Classifier interface {
	Classify(data []float64) (float64, error)
	Train(data [][]float64, correct []float64) error
}
