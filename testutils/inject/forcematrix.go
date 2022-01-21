package inject

import (
	"context"

	"go.viam.com/rdk/component/forcematrix"
)

// ForceMatrix is an injected ForceMatrix.
type ForceMatrix struct {
	forcematrix.ForceMatrix
	MatrixFunc     func(ctx context.Context) ([][]int, error)
	DetectSlipFunc func(ctx context.Context) (bool, error)
	ReadingsFunc   func(ctx context.Context) ([]interface{}, error)
}

// Matrix calls the injected MatrixFunc or the real variant.
func (m *ForceMatrix) Matrix(ctx context.Context) ([][]int, error) {
	if m.MatrixFunc == nil {
		return m.ForceMatrix.Matrix(ctx)
	}
	return m.MatrixFunc(ctx)
}

// DetectSlip calls the injected DetectSlipFunc or the real variant.
func (m *ForceMatrix) DetectSlip(ctx context.Context) (bool, error) {
	if m.DetectSlipFunc == nil {
		return m.ForceMatrix.DetectSlip(ctx)
	}
	return m.DetectSlipFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (m *ForceMatrix) Readings(ctx context.Context) ([]interface{}, error) {
	if m.ReadingsFunc == nil {
		return m.ForceMatrix.Readings(ctx)
	}
	return m.ReadingsFunc(ctx)
}
