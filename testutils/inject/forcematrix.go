package inject

import (
	"context"

	"go.viam.com/rdk/component/forcematrix"
)

// ForceMatrix is an injected ForceMatrix.
type ForceMatrix struct {
	forcematrix.ForceMatrix
	ReadMatrixFunc  func(ctx context.Context) ([][]int, error)
	DetectSlipFunc  func(ctx context.Context) (bool, error)
	GetReadingsFunc func(ctx context.Context) ([]interface{}, error)
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

// GetReadings calls the injected GetReadings or the real version.
func (m *ForceMatrix) GetReadings(ctx context.Context) ([]interface{}, error) {
	if m.GetReadingsFunc == nil {
		return m.ForceMatrix.GetReadings(ctx)
	}
	return m.GetReadingsFunc(ctx)
}
