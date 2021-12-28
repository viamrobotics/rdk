package inject

import (
	"context"

	"go.viam.com/rdk/component/forcematrix"
	"go.viam.com/rdk/sensor"
)

// ForceMatrix is an injected ForceMatrix.
type ForceMatrix struct {
	forcematrix.ForceMatrix
	MatrixFunc     func(ctx context.Context) ([][]int, error)
	IsSlippingFunc func(ctx context.Context) (bool, error)
	ReadingsFunc   func(ctx context.Context) ([]interface{}, error)
	DescFunc       func() sensor.Description
}

// Matrix calls the injected MatrixFunc or the real variant.
func (m *ForceMatrix) Matrix(ctx context.Context) ([][]int, error) {
	if m.MatrixFunc == nil {
		return m.ForceMatrix.Matrix(ctx)
	}
	return m.MatrixFunc(ctx)
}

// IsSlipping calls the injected IsSlippingFunc or the real variant.
func (m *ForceMatrix) IsSlipping(ctx context.Context) (bool, error) {
	if m.IsSlippingFunc == nil {
		return m.ForceMatrix.IsSlipping(ctx)
	}
	return m.IsSlippingFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (m *ForceMatrix) Readings(ctx context.Context) ([]interface{}, error) {
	if m.ReadingsFunc == nil {
		return m.ForceMatrix.Readings(ctx)
	}
	return m.ReadingsFunc(ctx)
}

// Desc returns that this is a force matrix.
func (m *ForceMatrix) Desc() sensor.Description {
	if m.DescFunc == nil {
		return m.ForceMatrix.Desc()
	}
	return m.DescFunc()
}
