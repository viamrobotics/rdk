package inject

import (
	"context"
	"errors"

	commonpb "go.viam.com/api/common/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// WorldStateStoreService is an injectable world object store service.
type WorldStateStoreService struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	name                       resource.Name
	ListUUIDsFunc              func(ctx context.Context, extra map[string]any) ([][]byte, error)
	GetTransformFunc           func(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error)
	StreamTransformChangesFunc func(ctx context.Context, extra map[string]any) (*worldstatestore.TransformChangeStream, error)
	DoFunc                     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
}

// NewWorldStateStoreService returns a new injected world state store service.
func NewWorldStateStoreService(name string) *WorldStateStoreService {
	return &WorldStateStoreService{name: worldstatestore.Named(name)}
}

// Name returns the name of the resource.
func (wosSvc *WorldStateStoreService) Name() resource.Name {
	return wosSvc.name
}

// ListUUIDs calls the injected ListUUIDsFunc or the real version.
func (wosSvc *WorldStateStoreService) ListUUIDs(ctx context.Context, extra map[string]any) ([][]byte, error) {
	if wosSvc.ListUUIDsFunc == nil {
		return nil, errors.New("ListUUIDsFunc not set")
	}
	return wosSvc.ListUUIDsFunc(ctx, extra)
}

// GetTransform calls the injected GetTransformFunc or the real version.
func (wosSvc *WorldStateStoreService) GetTransform(ctx context.Context, uuid []byte, extra map[string]any) (*commonpb.Transform, error) {
	if wosSvc.GetTransformFunc == nil {
		return nil, errors.New("GetTransformFunc not set")
	}
	return wosSvc.GetTransformFunc(ctx, uuid, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (wosSvc *WorldStateStoreService) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if wosSvc.DoFunc == nil {
		return nil, errors.New("DoCommandFunc not set")
	}
	return wosSvc.DoFunc(ctx, cmd)
}

// StreamTransformChanges calls the injected StreamTransformChangesFunc or the real version.
func (wosSvc *WorldStateStoreService) StreamTransformChanges(
	ctx context.Context,
	extra map[string]any,
) (*worldstatestore.TransformChangeStream, error) {
	if wosSvc.StreamTransformChangesFunc == nil {
		return nil, errors.New("StreamTransformChangesFunc not set")
	}
	return wosSvc.StreamTransformChangesFunc(ctx, extra)
}
