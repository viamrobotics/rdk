package inject

import (
	"context"
	"time"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	name              resource.Name
	PositionFunc      func(ctx context.Context) (spatialmath.Pose, string, error)
	PointCloudMapFunc func(ctx context.Context) (func() ([]byte, error), error)
	InternalStateFunc func(ctx context.Context) (func() ([]byte, error), error)
	LatestMapInfoFunc func(ctx context.Context) (time.Time, error)
	PropertiesFunc    func(ctx context.Context) (slam.Properties, error)
	DoCommandFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	CloseFunc         func(ctx context.Context) error
}

// NewSLAMService returns a new injected SLAM service.
func NewSLAMService(name string) *SLAMService {
	return &SLAMService{name: slam.Named(name)}
}

// Name returns the name of the resource.
func (slamSvc *SLAMService) Name() resource.Name {
	return slamSvc.name
}

// Position calls the injected PositionFunc or the real version.
func (slamSvc *SLAMService) Position(ctx context.Context) (spatialmath.Pose, string, error) {
	if slamSvc.PositionFunc == nil {
		return slamSvc.Service.Position(ctx)
	}
	return slamSvc.PositionFunc(ctx)
}

// PointCloudMap calls the injected PointCloudMap or the real version.
func (slamSvc *SLAMService) PointCloudMap(ctx context.Context) (func() ([]byte, error), error) {
	if slamSvc.PointCloudMapFunc == nil {
		return slamSvc.Service.PointCloudMap(ctx)
	}
	return slamSvc.PointCloudMapFunc(ctx)
}

// InternalState calls the injected InternalState or the real version.
func (slamSvc *SLAMService) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	if slamSvc.InternalStateFunc == nil {
		return slamSvc.Service.InternalState(ctx)
	}
	return slamSvc.InternalStateFunc(ctx)
}

// Properties calls the injected PropertiesFunc or the real version.
func (slamSvc *SLAMService) Properties(ctx context.Context) (slam.Properties, error) {
	if slamSvc.PropertiesFunc == nil {
		return slamSvc.Service.Properties(ctx)
	}
	return slamSvc.PropertiesFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real variant.
func (slamSvc *SLAMService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if slamSvc.DoCommandFunc == nil {
		return slamSvc.Service.DoCommand(ctx, cmd)
	}
	return slamSvc.DoCommandFunc(ctx, cmd)
}

// Close calls the injected Close or the real version.
func (slamSvc *SLAMService) Close(ctx context.Context) error {
	if slamSvc.CloseFunc == nil {
		if slamSvc.Service == nil {
			return nil
		}
		return slamSvc.Service.Close(ctx)
	}
	return slamSvc.CloseFunc(ctx)
}
