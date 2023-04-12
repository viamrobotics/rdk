package inject

import (
	"context"

	"go.viam.com/rdk/services/mlmodel"
)

// MLModelService represents a fake instance of a mlmodel service.
type MLModelService struct {
	mlmodel.Service
	InferFunc    func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	MetadataFunc func(ctx context.Context) (mlmodel.MLMetadata, error)
}

// Infer calls the injected InferFunc or the real version.
func (mlmodelSvc *MLModelService) Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if mlmodelSvc.InferFunc == nil {
		return mlmodelSvc.Service.Infer(ctx, input)
	}
	return mlmodelSvc.InferFunc(ctx, input)
}

// Metadata calls the injected MetadataFunc or the real version.
func (mlmodelSvc *MLModelService) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	if mlmodelSvc.MetadataFunc == nil {
		return mlmodelSvc.Service.Metadata(ctx)
	}
	return mlmodelSvc.MetadataFunc(ctx)
}
