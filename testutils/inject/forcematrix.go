package inject

import (
	"context"

	"go.viam.com/core/sensor/forcematrix"
)

// Forcematrix is an injected Forcematrix
type Forcematrix struct {
	forcematrix.Forcematrix
	MatrixFunc func(ctx context.Context) (matrix [][]int, err error)
}

// Matrix calls the injected MatrixFunc or the real variant
func (m *Forcematrix) Matrix(ctx context.Context) (matrix [][]int, err error) {
	if m.MatrixFunc == nil {
		return m.Forcematrix.Matrix(ctx)
	}
	return m.MatrixFunc(ctx)
}
