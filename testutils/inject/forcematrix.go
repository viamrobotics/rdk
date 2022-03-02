package inject

import (
	"context"

	"go.viam.com/rdk/component/forcematrix"
)

// ForceMatrix is an injected ForceMatrix.
type ForceMatrix struct {
	forcematrix.ForceMatrix
	ReadMatrixFunc func(ctx context.Context) ([][]int, error)
	DetectSlipFunc func(ctx context.Context) (bool, error)
}

// ReadMatrix calls the injected ReadMatrixFunc or the real variant.
func (m *ForceMatrix) ReadMatrix(ctx context.Context) ([][]int, error) {
	if m.ReadMatrixFunc == nil {
		return m.ForceMatrix.ReadMatrix(ctx)
	}
	return m.ReadMatrixFunc(ctx)
}

// DetectSlip calls the injected DetectSlipFunc or the real variant.
func (m *ForceMatrix) DetectSlip(ctx context.Context) (bool, error) {
	if m.DetectSlipFunc == nil {
		return m.ForceMatrix.DetectSlip(ctx)
	}
	return m.DetectSlipFunc(ctx)
}
