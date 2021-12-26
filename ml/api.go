// Package ml provides some fundamental machine learning primitives.
package ml

// TODO(erh): this is all wrong, I just need a pivot point in the sand

// Classifier TODO.
type Classifier interface {
	Classify(data []float64) (int, error)
	Train(data [][]float64, correct []int) error
}
