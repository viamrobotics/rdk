package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

// SLAMService represents a fake instance of a slam service.
type SLAMService struct {
	slam.Service
	name              resource.Name
	PositionFunc      func(ctx context.Context) (spatialmath.Pose, error)
	PointCloudMapFunc func(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error)
	InternalStateFunc func(ctx context.Context) (func() ([]byte, error), error)
	PropertiesFunc    func(ctx context.Context) (slam.Properties, error)
	DoCommandFunc     func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc        func(ctx context.Context) (map[string]interface{}, error)
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
func (slamSvc *SLAMService) Position(ctx context.Context) (spatialmath.Pose, error) {
	if slamSvc.PositionFunc == nil {
		return errtrace.Wrap2(slamSvc.Service.Position(ctx))
	}
	return errtrace.Wrap2(slamSvc.PositionFunc(ctx))
}

// PointCloudMap calls the injected PointCloudMap or the real version.
func (slamSvc *SLAMService) PointCloudMap(ctx context.Context, returnEditedMap bool) (func() ([]byte, error), error) {
	if slamSvc.PointCloudMapFunc == nil {
		return errtrace.Wrap2(slamSvc.Service.PointCloudMap(ctx, returnEditedMap))
	}
	return errtrace.Wrap2(slamSvc.PointCloudMapFunc(ctx, returnEditedMap))
}

// InternalState calls the injected InternalState or the real version.
func (slamSvc *SLAMService) InternalState(ctx context.Context) (func() ([]byte, error), error) {
	if slamSvc.InternalStateFunc == nil {
		return errtrace.Wrap2(slamSvc.Service.InternalState(ctx))
	}
	return errtrace.Wrap2(slamSvc.InternalStateFunc(ctx))
}

// Properties calls the injected PropertiesFunc or the real version.
func (slamSvc *SLAMService) Properties(ctx context.Context) (slam.Properties, error) {
	if slamSvc.PropertiesFunc == nil {
		return errtrace.Wrap2(slamSvc.Service.Properties(ctx))
	}
	return errtrace.Wrap2(slamSvc.PropertiesFunc(ctx))
}

// DoCommand calls the injected DoCommand or the real variant.
func (slamSvc *SLAMService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if slamSvc.DoCommandFunc == nil {
		return errtrace.Wrap2(slamSvc.Service.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(slamSvc.DoCommandFunc(ctx, cmd))
}

// Status calls the injected Status or the real version.
func (slamSvc *SLAMService) Status(ctx context.Context) (map[string]interface{}, error) {
	if slamSvc.StatusFunc != nil {
		return errtrace.Wrap2(slamSvc.StatusFunc(ctx))
	}
	if slamSvc.Service != nil {
		return errtrace.Wrap2(slamSvc.Service.Status(ctx))
	}
	return map[string]interface{}{}, nil
}

// Close calls the injected Close or the real version.
func (slamSvc *SLAMService) Close(ctx context.Context) error {
	if slamSvc.CloseFunc == nil {
		if slamSvc.Service == nil {
			return nil
		}
		return errtrace.Wrap(slamSvc.Service.Close(ctx))
	}
	return errtrace.Wrap(slamSvc.CloseFunc(ctx))
}
