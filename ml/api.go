package ml

// TODO(erh): this is all wrong, I just need a pivot point in the sand

type Classifier interface {
	Classify(data []float64) (int, error)
	Train(data [][]float64, correct []int) error
}
