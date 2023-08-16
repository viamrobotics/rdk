// Package ml provides some fundamental machine learning primitives.
package ml

import "gorgonia.org/tensor"

// Tensors are a data structure to hold the input and output map of tensors to be fed into a
// model or the result coming from the model.
type Tensors map[string]*tensor.Dense

// TODO(erh): this is all wrong, I just need a pivot point in the sand

// Classifier TODO.
type Classifier interface {
	Classify(data []float64) (int, error)
	Train(data [][]float64, correct []int) error
}
