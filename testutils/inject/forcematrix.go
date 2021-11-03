package inject

import (
	"context"

	"go.viam.com/core/sensor"
	"go.viam.com/core/sensor/forcematrix"
)

// ForceMatrix is an injected ForceMatrix
type ForceMatrix struct {
	forcematrix.ForceMatrix
	MatrixFunc     func(ctx context.Context) ([][]int, error)
	IsSlippingFunc func(ctx context.Context) (bool, error)
}

// Matrix calls the injected MatrixFunc or the real variant
func (m *ForceMatrix) Matrix(ctx context.Context) ([][]int, error) {
	if m.MatrixFunc == nil {
		return m.ForceMatrix.Matrix(ctx)
	}
	return m.MatrixFunc(ctx)
}

// IsSlipping calls the injected IsSlippingFunc or the real variant
func (m *ForceMatrix) IsSlipping(ctx context.Context) (bool, error) {
	if m.IsSlippingFunc == nil {
		return m.ForceMatrix.IsSlipping(ctx)
	}
	return m.IsSlippingFunc(ctx)
}

// Desc returns that this is a force matrix.
func (m *ForceMatrix) Desc() sensor.Description {
	return sensor.Description{Type: forcematrix.Type}
}
