package inject

import (
	"context"

	"go.viam.com/core/sensor/forcematrix"
)

// ForceMatrix is an injected ForceMatrix
type ForceMatrix struct {
	forcematrix.ForceMatrix
	MatrixFunc func(ctx context.Context) (matrix [][]int, err error)
}

// Matrix calls the injected MatrixFunc or the real variant
func (m *ForceMatrix) Matrix(ctx context.Context) (matrix [][]int, err error) {
	if m.MatrixFunc == nil {
		return m.ForceMatrix.Matrix(ctx)
	}
	return m.MatrixFunc(ctx)
}
