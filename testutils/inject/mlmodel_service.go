package inject

import (
	"context"

	"go.viam.com/rdk/ml"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
)

// MLModelService represents a fake instance of an MLModel service.
type MLModelService struct {
	mlmodel.Service
	name         resource.Name
	InferFunc    func(ctx context.Context, tensors ml.Tensors, input map[string]interface{}) (ml.Tensors, map[string]interface{}, error)
	MetadataFunc func(ctx context.Context) (mlmodel.MLMetadata, error)
	CloseFunc    func(ctx context.Context) error
}

// NewMLModelService returns a new injected mlmodel service.
func NewMLModelService(name string) *MLModelService {
	return &MLModelService{name: mlmodel.Named(name)}
}

// Name returns the name of the resource.
func (s *MLModelService) Name() resource.Name {
	return s.name
}

// Infer calls the injected Infer or the real variant.
func (s *MLModelService) Infer(
	ctx context.Context,
	tensors ml.Tensors,
	input map[string]interface{},
) (ml.Tensors, map[string]interface{}, error) {
	if s.InferFunc == nil {
		return s.Service.Infer(ctx, tensors, input)
	}
	return s.InferFunc(ctx, tensors, input)
}

// Metadata calls the injected Metadata or the real variant.
func (s *MLModelService) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	if s.MetadataFunc == nil {
		return s.Service.Metadata(ctx)
	}
	return s.MetadataFunc(ctx)
}

// Close calls the injected Close or the real version.
func (s *MLModelService) Close(ctx context.Context) error {
	if s.CloseFunc == nil {
		if s.Service == nil {
			return nil
		}
		return s.Service.Close(ctx)
	}
	return s.CloseFunc(ctx)
}
